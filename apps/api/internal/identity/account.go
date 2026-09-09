package identity

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"regexp"
	"strings"
	"time"
)

var ErrAccountRate = errors.New("account attempt limit reached")

var ErrAccountInput = errors.New("invalid account input")
var ErrUsernameTaken = errors.New("username unavailable")
var ErrUserOffline = errors.New("user offline or unavailable")
var usernamePattern = regexp.MustCompile(`^[a-z0-9_]{3,24}$`)

type Profile struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	DisplayName   string `json:"displayName"`
	Avatar        string `json:"avatar"`
	Online        bool   `json:"online"`
	Following     bool   `json:"following"`
	Friends       bool   `json:"friends"`
	ParticipantID string `json:"participantId,omitempty"`
}

type AccountRecord struct {
	Profile
	SessionID    string
	Salt         []byte
	PasswordHash []byte
}

type AccountLogin struct {
	Profile   Profile   `json:"profile"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
	SessionID string    `json:"sessionId"`
}

type Invitation struct {
	Sender     Profile   `json:"sender"`
	TableID    string    `json:"tableId"`
	InviteCode string    `json:"inviteCode"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

type AccountRepository interface {
	StoreAccountTicket(context.Context, []byte, string, []byte, time.Time, time.Time) error
	CreateAccount(context.Context, AccountRecord, SessionRecord) error
	FindAccount(context.Context, string) (AccountRecord, error)
	StoreAccountSession(context.Context, []byte, string, time.Time) error
	AuthenticateAccount(context.Context, []byte, time.Time) (AccountRecord, time.Time, error)
	RevokeAccountSession(context.Context, []byte) error
	UpdateProfile(context.Context, string, string, string) (Profile, error)
	Heartbeat(context.Context, []byte, time.Time) error
	SearchUsers(context.Context, string, string, bool, time.Time) ([]Profile, error)
	Follow(context.Context, string, string, bool) error
	ParticipantProfiles(context.Context, string, string, time.Time) ([]Profile, error)
	InvitePlayer(context.Context, string, string, string, string, time.Time) error
	Invitations(context.Context, string, time.Time) ([]Invitation, error)
}

func ValidAvatar(avatar string) bool {
	switch avatar {
	case "spade", "heart", "diamond", "club", "owl", "fox":
		return true
	}
	return false
}

func (service *Service) Register(ctx context.Context, username, password, displayName, avatar string) (AccountLogin, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if !service.allowAccountAttempt(username) {
		return AccountLogin{}, ErrAccountRate
	}
	name, err := normalizeNickname(displayName)
	if err != nil || !usernamePattern.MatchString(username) || len(password) < 10 || len(password) > 128 || !ValidAvatar(avatar) {
		return AccountLogin{}, ErrAccountInput
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return AccountLogin{}, err
	}
	hash, err := pbkdf2.Key(sha256.New, password, salt, 600000, 32)
	if err != nil {
		return AccountLogin{}, err
	}
	userID, err := randomUUID(service.random)
	if err != nil {
		return AccountLogin{}, err
	}
	sessionID, err := randomUUID(service.random)
	if err != nil {
		return AccountLogin{}, err
	}
	now := service.now().UTC()
	account := AccountRecord{Profile: Profile{ID: userID, Username: username, DisplayName: name, Avatar: avatar}, SessionID: sessionID, Salt: salt, PasswordHash: hash}
	repository, ok := service.repository.(AccountRepository)
	if !ok {
		return AccountLogin{}, ErrSessionInactive
	}
	err = repository.CreateAccount(ctx, account, SessionRecord{Session: Session{ID: sessionID, Nickname: name, Status: "ACTIVE", ExpiresAt: now.AddDate(100, 0, 0)}, CredentialHash: service.hash("unusable-account-device", rand.Text()), CreatedAt: now, LastSeenAt: now})
	if err != nil {
		return AccountLogin{}, err
	}
	return service.accountLogin(ctx, account)
}

func (service *Service) Login(ctx context.Context, username, password string) (AccountLogin, error) {
	if !service.allowAccountAttempt(strings.ToLower(strings.TrimSpace(username))) {
		return AccountLogin{}, ErrAccountRate
	}
	if len(password) > 128 || len(username) > 24 {
		return AccountLogin{}, ErrInvalidCredential
	}
	repository, ok := service.repository.(AccountRepository)
	if !ok {
		return AccountLogin{}, ErrSessionInactive
	}
	account, findErr := repository.FindAccount(ctx, strings.ToLower(strings.TrimSpace(username)))
	salt := account.Salt
	if findErr != nil {
		salt = make([]byte, 16)
	}
	hash, err := pbkdf2.Key(sha256.New, password, salt, 600000, 32)
	if err != nil || findErr != nil || subtle.ConstantTimeCompare(hash, account.PasswordHash) != 1 {
		return AccountLogin{}, ErrInvalidCredential
	}
	return service.accountLogin(ctx, account)
}

func (service *Service) accountLogin(ctx context.Context, account AccountRecord) (AccountLogin, error) {
	token, err := randomToken(service.random, 32)
	if err != nil {
		return AccountLogin{}, err
	}
	token = "acct_" + token
	expiresAt := service.now().UTC().Add(sessionLifetime)
	repository := service.repository.(AccountRepository)
	if err := repository.StoreAccountSession(ctx, service.hash("account", token), account.ID, expiresAt); err != nil {
		return AccountLogin{}, err
	}
	return AccountLogin{Profile: account.Profile, Token: token, ExpiresAt: expiresAt, SessionID: account.SessionID}, nil
}

func (service *Service) Account(ctx context.Context, token string) (AccountLogin, error) {
	repository, ok := service.repository.(AccountRepository)
	if !ok || !strings.HasPrefix(token, "acct_") || !validOpaqueToken(strings.TrimPrefix(token, "acct_"), 32) {
		return AccountLogin{}, ErrInvalidCredential
	}
	account, expiresAt, err := repository.AuthenticateAccount(ctx, service.hash("account", token), service.now().UTC())
	if err != nil {
		return AccountLogin{}, err
	}
	return AccountLogin{Profile: account.Profile, Token: token, ExpiresAt: expiresAt, SessionID: account.SessionID}, nil
}

func (service *Service) LogoutAccount(ctx context.Context, token string) error {
	repository, ok := service.repository.(AccountRepository)
	if !ok {
		return ErrInvalidCredential
	}
	return repository.RevokeAccountSession(ctx, service.hash("account", token))
}

func (service *Service) AccountHeartbeat(ctx context.Context, token string) error {
	repository, ok := service.repository.(AccountRepository)
	if !ok {
		return ErrInvalidCredential
	}
	return repository.Heartbeat(ctx, service.hash("account", token), service.now().UTC())
}

func (service *Service) SaveProfile(ctx context.Context, userID, displayName, avatar string) (Profile, error) {
	name, err := normalizeNickname(displayName)
	if err != nil || !ValidAvatar(avatar) {
		return Profile{}, ErrAccountInput
	}
	repository, ok := service.repository.(AccountRepository)
	if !ok {
		return Profile{}, ErrInvalidCredential
	}
	return repository.UpdateProfile(ctx, userID, name, avatar)
}

func (service *Service) allowAccountAttempt(username string) bool {
	service.accountAuthMu.Lock()
	defer service.accountAuthMu.Unlock()
	now := service.now().UTC()
	if service.accountAuthWindow.IsZero() || now.Sub(service.accountAuthWindow) >= time.Minute {
		service.accountAuthWindow = now
		service.accountAuthCount = 0
		service.accountAuthUsers = make(map[string]int)
	}
	if service.accountAuthCount >= 120 || service.accountAuthUsers[username] >= 10 {
		return false
	}
	service.accountAuthCount++
	service.accountAuthUsers[username]++
	return true
}

func (service *Service) ValidateConnection(ctx context.Context, session Session) error {
	if len(session.AccountTokenHash) > 0 {
		repository, ok := service.repository.(AccountRepository)
		if !ok {
			return ErrInvalidCredential
		}
		_, _, err := repository.AuthenticateAccount(ctx, session.AccountTokenHash, service.now().UTC())
		if err != nil {
			return err
		}
	}
	_, err := service.ValidateSession(ctx, session.ID)
	return err
}
