package identity

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type memoryRepository struct {
	session        Session
	credentialHash []byte
	ticketHash     []byte
	ticketUsed     bool
}

func (repository *memoryRepository) CreateSession(_ context.Context, record SessionRecord) error {
	repository.session = record.Session
	repository.credentialHash = append([]byte(nil), record.CredentialHash...)
	return nil
}

func (repository *memoryRepository) RotateCredential(_ context.Context, oldHash []byte, newHash []byte, now time.Time) (Session, error) {
	if !equalBytes(oldHash, repository.credentialHash) || !now.Before(repository.session.ExpiresAt) || repository.session.Status != "ACTIVE" {
		return Session{}, ErrInvalidCredential
	}
	repository.credentialHash = append([]byte(nil), newHash...)
	return repository.session, nil
}

func (repository *memoryRepository) FindActiveSession(_ context.Context, id string, now time.Time) (Session, error) {
	if id != repository.session.ID || !now.Before(repository.session.ExpiresAt) || repository.session.Status != "ACTIVE" {
		return Session{}, ErrSessionInactive
	}
	return repository.session, nil
}

func (repository *memoryRepository) RevokeSession(_ context.Context, id string, _ time.Time) error {
	if id != repository.session.ID {
		return ErrSessionInactive
	}
	repository.session.Status = "REVOKED"
	return nil
}

func (repository *memoryRepository) StoreTicket(_ context.Context, hash []byte, sessionID string, _, _ time.Time) error {
	if sessionID != repository.session.ID {
		return ErrSessionInactive
	}
	repository.ticketHash = append([]byte(nil), hash...)
	repository.ticketUsed = false
	return nil
}

func (repository *memoryRepository) ConsumeTicket(_ context.Context, hash []byte, now time.Time) (Session, error) {
	if !equalBytes(hash, repository.ticketHash) || repository.ticketUsed || !now.Before(repository.session.ExpiresAt) {
		return Session{}, ErrInvalidTicket
	}
	repository.ticketUsed = true
	return repository.session, nil
}

func TestServiceCreateAuthenticateRefreshAndRevoke(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 30, 4, 0, 0, 0, time.UTC)
	repository := &memoryRepository{}
	service, err := NewService(repository, []byte(strings.Repeat("p", 32)), bytes.NewReader(deterministicBytes(256)), func() time.Time { return now })
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	credentials, err := service.CreateSession(context.Background(), "  Mahesa   Yok  ")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if credentials.Session.Nickname != "Mahesa Yok" {
		t.Fatalf("nickname = %q, want %q", credentials.Session.Nickname, "Mahesa Yok")
	}
	if _, err := service.Authenticate(context.Background(), credentials.AccessToken); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	refreshed, err := service.Refresh(context.Background(), credentials.DeviceCredential)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if refreshed.DeviceCredential == credentials.DeviceCredential {
		t.Fatal("Refresh() did not rotate the device credential")
	}
	if _, err := service.Refresh(context.Background(), credentials.DeviceCredential); err == nil {
		t.Fatal("old device credential remained valid after rotation")
	}
	if err := service.Revoke(context.Background(), credentials.Session.ID); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if _, err := service.Authenticate(context.Background(), refreshed.AccessToken); err == nil {
		t.Fatal("revoked session authenticated")
	}
}

func TestServiceTicketIsSingleUse(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 30, 4, 0, 0, 0, time.UTC)
	repository := &memoryRepository{}
	service, err := NewService(repository, []byte(strings.Repeat("p", 32)), bytes.NewReader(deterministicBytes(256)), func() time.Time { return now })
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	credentials, err := service.CreateSession(context.Background(), "North Player")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	ticket, _, err := service.IssueTicket(context.Background(), credentials.Session)
	if err != nil {
		t.Fatalf("IssueTicket() error = %v", err)
	}
	if _, err := service.ConsumeTicket(context.Background(), ticket); err != nil {
		t.Fatalf("ConsumeTicket() error = %v", err)
	}
	if _, err := service.ConsumeTicket(context.Background(), ticket); err == nil {
		t.Fatal("ticket replay succeeded")
	}
}

func TestServiceValidateSessionReadsCurrentDurableState(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 30, 4, 0, 0, 0, time.UTC)
	repository := &memoryRepository{}
	service, err := NewService(repository, []byte(strings.Repeat("p", 32)), bytes.NewReader(deterministicBytes(256)), func() time.Time { return now })
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	credentials, err := service.CreateSession(context.Background(), "North Player")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if _, err := service.ValidateSession(context.Background(), credentials.Session.ID); err != nil {
		t.Fatalf("ValidateSession() error = %v", err)
	}
	repository.session.Status = "REVOKED"
	if _, err := service.ValidateSession(context.Background(), credentials.Session.ID); err == nil {
		t.Fatal("ValidateSession() accepted revoked durable session")
	}
	if _, err := service.ValidateSession(context.Background(), "invalid"); !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("invalid ValidateSession() error = %v", err)
	}
}

func TestNormalizeNickname(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    string
		wantErr error
	}{
		{name: "collapse whitespace", value: "  North   Star ", want: "North Star"},
		{name: "count emoji grapheme", value: "👨‍👩‍👧‍👦 A", want: "👨‍👩‍👧‍👦 A"},
		{name: "too short", value: "A", wantErr: ErrInvalidNickname},
		{name: "control character", value: "North\nSouth", wantErr: ErrInvalidNickname},
		{name: "bidi override", value: "North\u202eSouth", wantErr: ErrInvalidNickname},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeNickname(test.value)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("normalizeNickname() error = %v, want %v", err, test.wantErr)
			}
			if got != test.want {
				t.Fatalf("normalizeNickname() = %q, want %q", got, test.want)
			}
		})
	}
}

func equalBytes(left []byte, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for _index := range left {
		if left[_index] != right[_index] {
			return false
		}
	}
	return true
}

func deterministicBytes(length int) []byte {
	value := make([]byte, length)
	for _index := range value {
		value[_index] = byte(_index)
	}
	return value
}
