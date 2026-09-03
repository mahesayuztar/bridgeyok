package table

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
)

const inviteCreationAttempts = 3

var (
	ErrInviteCollision  = errors.New("invite code collision")
	ErrTableNotFound    = errors.New("table not found")
	ErrTableUnavailable = errors.New("table unavailable")
)

// Preview is the public, non-joining view of an invite destination.
type Preview struct {
	State            State
	Locked           bool
	ParticipantCount int
	Capacity         int
}

// CreateRecord contains one table and its non-recoverable invite verifier.
type CreateRecord struct {
	Aggregate      Aggregate
	InviteCodeHash []byte
	CreatedAt      time.Time
}

// CreatedTable contains the owner projection and the invite secret shown once.
type CreatedTable struct {
	InviteCode string
	Projection Projection
}

// Repository is the durable lifecycle boundary consumed by Service.
type Repository interface {
	CreateTable(context.Context, CreateRecord) error
	PreviewTable(context.Context, []byte) (Preview, error)
	JoinTable(context.Context, []byte, Participant) (Aggregate, error)
	FindTable(context.Context, string) (Aggregate, error)
	LeaveTable(context.Context, string, string, time.Time) error
}

// Service creates and resolves private table resources.
type Service struct {
	repository Repository
	pepper     []byte
	random     io.Reader
	now        func() time.Time
}

// NewService constructs table lifecycle operations with explicit security dependencies.
func NewService(repository Repository, pepper []byte, random io.Reader, now func() time.Time) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("table repository is required")
	}
	if len(pepper) < 32 {
		return nil, fmt.Errorf("table pepper must contain at least 32 bytes")
	}
	if random == nil {
		return nil, fmt.Errorf("table random source is required")
	}
	if now == nil {
		return nil, fmt.Errorf("table clock is required")
	}
	return &Service{repository: repository, pepper: append([]byte(nil), pepper...), random: random, now: now}, nil
}

// Create creates a waiting table with the authenticated guest as owner.
func (service *Service) Create(ctx context.Context, session identity.Session) (CreatedTable, error) {
	for _attempt := 0; _attempt < inviteCreationAttempts; _attempt++ {
		tableID, err := service.randomUUID()
		if err != nil {
			return CreatedTable{}, fmt.Errorf("generate table id: %w", err)
		}
		participantID, err := service.randomUUID()
		if err != nil {
			return CreatedTable{}, fmt.Errorf("generate owner participant id: %w", err)
		}
		inviteCode, err := service.randomInviteCode()
		if err != nil {
			return CreatedTable{}, fmt.Errorf("generate invite code: %w", err)
		}
		now := service.now().UTC()
		aggregate, err := NewAggregate(tableID, Participant{
			ID:        participantID,
			SessionID: session.ID,
			Nickname:  session.Nickname,
			Role:      RoleOwner,
			JoinedAt:  now,
		})
		if err != nil {
			return CreatedTable{}, fmt.Errorf("create table aggregate: %w", err)
		}
		err = service.repository.CreateTable(ctx, CreateRecord{
			Aggregate:      aggregate,
			InviteCodeHash: service.hashInvite(inviteCode),
			CreatedAt:      now,
		})
		if errors.Is(err, ErrInviteCollision) {
			continue
		}
		if err != nil {
			return CreatedTable{}, fmt.Errorf("persist table: %w", err)
		}
		projection, domainError := Project(aggregate, session.ID)
		if domainError != nil {
			return CreatedTable{}, fmt.Errorf("project created table: %w", domainError)
		}
		return CreatedTable{InviteCode: inviteCode, Projection: projection}, nil
	}
	return CreatedTable{}, fmt.Errorf("generate unique invite code: %w", ErrInviteCollision)
}

// Preview resolves a private invite without joining or exposing participant identity.
func (service *Service) Preview(ctx context.Context, inviteCode string) (Preview, error) {
	normalized, err := normalizeInviteCode(inviteCode)
	if err != nil {
		return Preview{}, ErrTableNotFound
	}
	preview, err := service.repository.PreviewTable(ctx, service.hashInvite(normalized))
	if err != nil {
		return Preview{}, fmt.Errorf("preview table: %w", err)
	}
	return preview, nil
}

// Join atomically adds an authenticated guest to an available waiting table.
func (service *Service) Join(ctx context.Context, inviteCode string, session identity.Session) (Projection, error) {
	normalized, err := normalizeInviteCode(inviteCode)
	if err != nil {
		return Projection{}, ErrTableUnavailable
	}
	participantID, err := service.randomUUID()
	if err != nil {
		return Projection{}, fmt.Errorf("generate participant id: %w", err)
	}
	aggregate, err := service.repository.JoinTable(ctx, service.hashInvite(normalized), Participant{
		ID:        participantID,
		SessionID: session.ID,
		Nickname:  session.Nickname,
		Role:      RoleParticipant,
		JoinedAt:  service.now().UTC(),
	})
	if err != nil {
		return Projection{}, fmt.Errorf("join table: %w", err)
	}
	projection, domainError := Project(aggregate, session.ID)
	if domainError != nil {
		return Projection{}, fmt.Errorf("project joined table: %w", domainError)
	}
	return projection, nil
}

// Get returns the recipient-projected state of one joined table.
func (service *Service) Get(ctx context.Context, tableID string, session identity.Session) (Projection, error) {
	if _, err := uuid.Parse(tableID); err != nil {
		return Projection{}, ErrTableNotFound
	}
	aggregate, err := service.repository.FindTable(ctx, tableID)
	if err != nil {
		return Projection{}, fmt.Errorf("find table: %w", err)
	}
	projection, domainError := Project(aggregate, session.ID)
	if domainError != nil {
		return Projection{}, ErrTableNotFound
	}
	return projection, nil
}

// Leave removes the authenticated participant, transferring ownership or closing the table when required.
func (service *Service) Leave(ctx context.Context, tableID string, session identity.Session) error {
	if _, err := uuid.Parse(tableID); err != nil {
		return ErrTableNotFound
	}
	if err := service.repository.LeaveTable(ctx, tableID, session.ID, service.now().UTC()); err != nil {
		return fmt.Errorf("leave table: %w", err)
	}
	return nil
}

func (service *Service) randomUUID() (string, error) {
	value, err := uuid.NewRandomFromReader(service.random)
	if err != nil {
		return "", err
	}
	return value.String(), nil
}

func (service *Service) randomInviteCode() (string, error) {
	value := make([]byte, 16)
	if _, err := io.ReadFull(service.random, value); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(value), nil
}

func (service *Service) hashInvite(inviteCode string) []byte {
	mac := hmac.New(sha256.New, service.pepper)
	_, _ = mac.Write([]byte("invite"))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(inviteCode))
	return mac.Sum(nil)
}

func normalizeInviteCode(inviteCode string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(inviteCode))
	if len(normalized) != 26 {
		return "", fmt.Errorf("invalid invite code length")
	}
	for _, character := range normalized {
		if character < 'A' || character > 'Z' {
			if character < '2' || character > '7' {
				return "", fmt.Errorf("invalid invite code character")
			}
		}
	}
	return normalized, nil
}
