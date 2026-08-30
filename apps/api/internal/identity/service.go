package identity

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	"github.com/rivo/uniseg"
)

const (
	accessLifetime  = 15 * time.Minute
	sessionLifetime = 30 * 24 * time.Hour
	ticketLifetime  = 45 * time.Second
)

var (
	ErrInvalidCredential = errors.New("invalid credential")
	ErrInvalidNickname   = errors.New("invalid nickname")
	ErrInvalidTicket     = errors.New("invalid realtime ticket")
	ErrSessionInactive   = errors.New("guest session is inactive")
)

// Session is one durable guest identity without its secret verifier.
type Session struct {
	ID        string
	Nickname  string
	Status    string
	ExpiresAt time.Time
}

// CredentialSet contains the secrets returned to a guest client.
type CredentialSet struct {
	Session          Session
	AccessToken      string
	AccessExpiresAt  time.Time
	DeviceCredential string
}

// SessionRecord contains the data required to persist a new guest session.
type SessionRecord struct {
	Session
	CredentialHash []byte
	CreatedAt      time.Time
	LastSeenAt     time.Time
}

// Repository is the durable identity boundary consumed by Service.
type Repository interface {
	CreateSession(context.Context, SessionRecord) error
	RotateCredential(context.Context, []byte, []byte, time.Time) (Session, error)
	FindActiveSession(context.Context, string, time.Time) (Session, error)
	RevokeSession(context.Context, string, time.Time) error
	StoreTicket(context.Context, []byte, string, time.Time, time.Time) error
	ConsumeTicket(context.Context, []byte, time.Time) (Session, error)
}

// Service issues and validates guest identity credentials.
type Service struct {
	repository Repository
	pepper     []byte
	random     io.Reader
	now        func() time.Time
}

// NewService constructs guest identity operations with explicit security dependencies.
func NewService(repository Repository, pepper []byte, random io.Reader, now func() time.Time) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("identity repository is required")
	}
	if len(pepper) < 32 {
		return nil, fmt.Errorf("identity pepper must contain at least 32 bytes")
	}
	if random == nil {
		return nil, fmt.Errorf("identity random source is required")
	}
	if now == nil {
		return nil, fmt.Errorf("identity clock is required")
	}
	return &Service{repository: repository, pepper: append([]byte(nil), pepper...), random: random, now: now}, nil
}

// CreateSession creates a guest and returns its one-time device credential.
func (service *Service) CreateSession(ctx context.Context, nickname string) (CredentialSet, error) {
	normalizedNickname, err := normalizeNickname(nickname)
	if err != nil {
		return CredentialSet{}, err
	}
	sessionID, err := randomUUID(service.random)
	if err != nil {
		return CredentialSet{}, fmt.Errorf("generate session id: %w", err)
	}
	deviceCredential, err := randomToken(service.random, 32)
	if err != nil {
		return CredentialSet{}, fmt.Errorf("generate device credential: %w", err)
	}
	now := service.now().UTC()
	session := Session{ID: sessionID, Nickname: normalizedNickname, Status: "ACTIVE", ExpiresAt: now.Add(sessionLifetime)}
	if err := service.repository.CreateSession(ctx, SessionRecord{
		Session:        session,
		CredentialHash: service.hash("device", deviceCredential),
		CreatedAt:      now,
		LastSeenAt:     now,
	}); err != nil {
		return CredentialSet{}, fmt.Errorf("create guest session: %w", err)
	}
	return service.credentials(session, deviceCredential, now)
}

// Refresh rotates a valid device credential and issues a fresh access token.
func (service *Service) Refresh(ctx context.Context, deviceCredential string) (CredentialSet, error) {
	if !validOpaqueToken(deviceCredential, 32) {
		return CredentialSet{}, ErrInvalidCredential
	}
	newDeviceCredential, err := randomToken(service.random, 32)
	if err != nil {
		return CredentialSet{}, fmt.Errorf("generate rotated device credential: %w", err)
	}
	now := service.now().UTC()
	session, err := service.repository.RotateCredential(
		ctx,
		service.hash("device", deviceCredential),
		service.hash("device", newDeviceCredential),
		now,
	)
	if err != nil {
		return CredentialSet{}, fmt.Errorf("rotate device credential: %w", err)
	}
	return service.credentials(session, newDeviceCredential, now)
}

// Authenticate validates an access token and the current durable session state.
func (service *Service) Authenticate(ctx context.Context, accessToken string) (Session, error) {
	parts := strings.Split(accessToken, ".")
	if len(parts) != 2 {
		return Session{}, ErrInvalidCredential
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Session{}, ErrInvalidCredential
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(signature, service.hash("access", string(payload))) {
		return Session{}, ErrInvalidCredential
	}
	payloadParts := strings.Split(string(payload), "|")
	if len(payloadParts) != 2 || !validUUID(payloadParts[0]) {
		return Session{}, ErrInvalidCredential
	}
	expiresAt, err := time.Parse(time.RFC3339, payloadParts[1])
	if err != nil || !service.now().UTC().Before(expiresAt) {
		return Session{}, ErrInvalidCredential
	}
	session, err := service.repository.FindActiveSession(ctx, payloadParts[0], service.now().UTC())
	if err != nil {
		return Session{}, fmt.Errorf("authenticate guest session: %w", err)
	}
	return session, nil
}

// Revoke invalidates a guest session and its future ticket or credential use.
func (service *Service) Revoke(ctx context.Context, sessionID string) error {
	if !validUUID(sessionID) {
		return ErrInvalidCredential
	}
	if err := service.repository.RevokeSession(ctx, sessionID, service.now().UTC()); err != nil {
		return fmt.Errorf("revoke guest session: %w", err)
	}
	return nil
}

// IssueTicket creates one short-lived single-use WebSocket ticket.
func (service *Service) IssueTicket(ctx context.Context, session Session) (string, time.Time, error) {
	ticket, err := randomToken(service.random, 32)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generate realtime ticket: %w", err)
	}
	now := service.now().UTC()
	expiresAt := now.Add(ticketLifetime)
	if err := service.repository.StoreTicket(ctx, service.hash("ticket", ticket), session.ID, now, expiresAt); err != nil {
		return "", time.Time{}, fmt.Errorf("store realtime ticket: %w", err)
	}
	return ticket, expiresAt, nil
}

// ConsumeTicket atomically exchanges one realtime ticket for its guest session.
func (service *Service) ConsumeTicket(ctx context.Context, ticket string) (Session, error) {
	if !validOpaqueToken(ticket, 32) {
		return Session{}, ErrInvalidTicket
	}
	session, err := service.repository.ConsumeTicket(ctx, service.hash("ticket", ticket), service.now().UTC())
	if err != nil {
		return Session{}, fmt.Errorf("consume realtime ticket: %w", err)
	}
	return session, nil
}

// ValidateSession resolves the current active state of a connected guest.
func (service *Service) ValidateSession(ctx context.Context, sessionID string) (Session, error) {
	if !validUUID(sessionID) {
		return Session{}, ErrInvalidCredential
	}
	session, err := service.repository.FindActiveSession(ctx, sessionID, service.now().UTC())
	if err != nil {
		return Session{}, fmt.Errorf("validate guest session: %w", err)
	}
	return session, nil
}

func (service *Service) credentials(session Session, deviceCredential string, now time.Time) (CredentialSet, error) {
	expiresAt := now.Add(accessLifetime)
	payload := base64.RawURLEncoding.EncodeToString([]byte(session.ID + "|" + expiresAt.Format(time.RFC3339)))
	signature := base64.RawURLEncoding.EncodeToString(service.hash("access", session.ID+"|"+expiresAt.Format(time.RFC3339)))
	return CredentialSet{
		Session:          session,
		AccessToken:      payload + "." + signature,
		AccessExpiresAt:  expiresAt,
		DeviceCredential: deviceCredential,
	}, nil
}

func (service *Service) hash(purpose string, value string) []byte {
	mac := hmac.New(sha256.New, service.pepper)
	_, _ = mac.Write([]byte(purpose))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

func normalizeNickname(raw string) (string, error) {
	for _, character := range raw {
		if unicode.IsControl(character) || character == '\u202A' || character == '\u202B' || character == '\u202D' || character == '\u202E' || character == '\u2066' || character == '\u2067' || character == '\u2068' || character == '\u2069' {
			return "", ErrInvalidNickname
		}
	}
	nickname := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	graphemes := uniseg.NewGraphemes(nickname)
	count := 0
	for graphemes.Next() {
		count++
	}
	if count < 2 || count > 24 {
		return "", ErrInvalidNickname
	}
	return nickname, nil
}

func randomToken(random io.Reader, bytes int) (string, error) {
	buffer := make([]byte, bytes)
	if _, err := io.ReadFull(random, buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func randomUUID(random io.Reader) (string, error) {
	var value [16]byte
	if _, err := io.ReadFull(random, value[:]); err != nil {
		return "", err
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	encoded := hex.EncodeToString(value[:])
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:], nil
}

func validUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	_, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	return err == nil
}

func validOpaqueToken(value string, expectedBytes int) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	return err == nil && len(decoded) == expectedBytes
}
