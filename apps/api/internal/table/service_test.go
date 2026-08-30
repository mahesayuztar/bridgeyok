package table

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
)

type serviceRepository struct {
	aggregate      Aggregate
	inviteCodeHash []byte
	createErrors   []error
	createCalls    int
}

func (repository *serviceRepository) CreateTable(_ context.Context, record CreateRecord) error {
	callIndex := repository.createCalls
	repository.createCalls++
	if callIndex < len(repository.createErrors) && repository.createErrors[callIndex] != nil {
		return repository.createErrors[callIndex]
	}
	repository.aggregate = record.Aggregate
	repository.inviteCodeHash = append([]byte(nil), record.InviteCodeHash...)
	return nil
}

func (repository *serviceRepository) PreviewTable(_ context.Context, inviteCodeHash []byte) (Preview, error) {
	if !bytes.Equal(inviteCodeHash, repository.inviteCodeHash) {
		return Preview{}, ErrTableNotFound
	}
	return Preview{State: repository.aggregate.State, Locked: repository.aggregate.Locked, ParticipantCount: repository.aggregate.activeParticipantCount(), Capacity: 4}, nil
}

func (repository *serviceRepository) JoinTable(_ context.Context, inviteCodeHash []byte, participant Participant) (Aggregate, error) {
	if !bytes.Equal(inviteCodeHash, repository.inviteCodeHash) {
		return Aggregate{}, ErrTableUnavailable
	}
	decision, domainError := Decide(repository.aggregate, Command{Name: CommandJoinTable, Participant: &participant})
	if domainError != nil {
		return Aggregate{}, ErrTableUnavailable
	}
	repository.aggregate = decision.NextState
	return repository.aggregate, nil
}

func (repository *serviceRepository) FindTable(_ context.Context, tableID string) (Aggregate, error) {
	if tableID != repository.aggregate.ID {
		return Aggregate{}, ErrTableNotFound
	}
	return repository.aggregate, nil
}

func (repository *serviceRepository) LeaveTable(_ context.Context, tableID string, sessionID string, occurredAt time.Time) error {
	if tableID != repository.aggregate.ID {
		return ErrTableNotFound
	}
	decision, domainError := Decide(repository.aggregate, Command{Name: CommandLeaveTable, SessionID: sessionID, OccurredAt: occurredAt})
	if domainError != nil {
		return domainError
	}
	repository.aggregate = decision.NextState
	return nil
}

func TestServiceTableLifecycle(t *testing.T) {
	t.Parallel()

	repository := &serviceRepository{}
	service := testTableService(t, repository)
	owner := identity.Session{ID: "0f4b9a5b-0ea8-4ad6-a866-e576ccd8be31", Nickname: "Owner"}
	created, err := service.Create(context.Background(), owner)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.InviteCode == "" || created.Projection.ViewerRole != RoleOwner || created.Projection.TableID == "" {
		t.Fatalf("Create() = %+v", created)
	}

	preview, err := service.Preview(context.Background(), strings.ToLower(created.InviteCode))
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}
	if preview.State != StateWaiting || preview.ParticipantCount != 1 || preview.Capacity != 4 {
		t.Fatalf("Preview() = %+v", preview)
	}

	guest := identity.Session{ID: "a3908d60-44dd-4887-a58b-606facce0a16", Nickname: "Guest"}
	joined, err := service.Join(context.Background(), created.InviteCode, guest)
	if err != nil {
		t.Fatalf("Join() error = %v", err)
	}
	if joined.ViewerRole != RoleParticipant || len(joined.Participants) != 2 {
		t.Fatalf("Join() = %+v", joined)
	}
	fetched, err := service.Get(context.Background(), joined.TableID, guest)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if fetched.ViewerParticipantID != joined.ViewerParticipantID {
		t.Fatalf("Get() viewer = %q, want %q", fetched.ViewerParticipantID, joined.ViewerParticipantID)
	}
	if err := service.Leave(context.Background(), joined.TableID, guest); err != nil {
		t.Fatalf("Leave() error = %v", err)
	}
	if _, err := service.Get(context.Background(), joined.TableID, guest); !errors.Is(err, ErrTableNotFound) {
		t.Fatalf("Get() after leave error = %v, want %v", err, ErrTableNotFound)
	}
}

func TestNewServiceRejectsInvalidDependencies(t *testing.T) {
	t.Parallel()

	validRepository := &serviceRepository{}
	validPepper := []byte(strings.Repeat("p", 32))
	validRandom := bytes.NewReader(make([]byte, 64))
	validNow := func() time.Time { return testJoinedAt }
	tests := []struct {
		name       string
		repository Repository
		pepper     []byte
		random     io.Reader
		now        func() time.Time
	}{
		{name: "repository", pepper: validPepper, random: validRandom, now: validNow},
		{name: "pepper", repository: validRepository, pepper: []byte("short"), random: validRandom, now: validNow},
		{name: "random", repository: validRepository, pepper: validPepper, now: validNow},
		{name: "clock", repository: validRepository, pepper: validPepper, random: validRandom},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewService(test.repository, test.pepper, test.random, test.now); err == nil {
				t.Fatal("NewService() error = nil")
			}
		})
	}
}

func TestServiceCreateRetriesInviteCollision(t *testing.T) {
	t.Parallel()

	repository := &serviceRepository{createErrors: []error{ErrInviteCollision, nil}}
	service := testTableService(t, repository)
	_, err := service.Create(context.Background(), identity.Session{ID: "0f4b9a5b-0ea8-4ad6-a866-e576ccd8be31", Nickname: "Owner"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if repository.createCalls != 2 {
		t.Fatalf("create calls = %d, want 2", repository.createCalls)
	}
}

func TestServiceCreateFailureBehavior(t *testing.T) {
	t.Parallel()

	owner := identity.Session{ID: "0f4b9a5b-0ea8-4ad6-a866-e576ccd8be31", Nickname: "Owner"}
	tests := []struct {
		name       string
		repository *serviceRepository
		randomErr  error
		wantError  error
	}{
		{name: "collision exhausted", repository: &serviceRepository{createErrors: []error{ErrInviteCollision, ErrInviteCollision, ErrInviteCollision}}, wantError: ErrInviteCollision},
		{name: "repository failure", repository: &serviceRepository{createErrors: []error{errors.New("database unavailable")}}, wantError: errors.New("database unavailable")},
		{name: "random failure", repository: &serviceRepository{}, randomErr: errors.New("random unavailable"), wantError: errors.New("random unavailable")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var randomSource io.Reader = bytes.NewReader(bytes.Repeat([]byte{0xff}, 1024))
			if test.randomErr != nil {
				randomSource = iotest.ErrReader(test.randomErr)
			}
			service, err := NewService(test.repository, []byte(strings.Repeat("p", 32)), randomSource, func() time.Time { return testJoinedAt })
			if err != nil {
				t.Fatalf("NewService() error = %v", err)
			}
			_, err = service.Create(context.Background(), owner)
			if err == nil || !strings.Contains(err.Error(), test.wantError.Error()) {
				t.Fatalf("Create() error = %v, want containing %q", err, test.wantError)
			}
		})
	}
}

func TestServiceSafeLookupErrors(t *testing.T) {
	t.Parallel()

	service := testTableService(t, &serviceRepository{})
	guest := identity.Session{ID: "a3908d60-44dd-4887-a58b-606facce0a16", Nickname: "Guest"}
	if _, err := service.Preview(context.Background(), "invalid"); !errors.Is(err, ErrTableNotFound) {
		t.Fatalf("Preview() error = %v, want %v", err, ErrTableNotFound)
	}
	if _, err := service.Join(context.Background(), "invalid", guest); !errors.Is(err, ErrTableUnavailable) {
		t.Fatalf("Join() error = %v, want %v", err, ErrTableUnavailable)
	}
	if _, err := service.Get(context.Background(), "invalid", guest); !errors.Is(err, ErrTableNotFound) {
		t.Fatalf("Get() error = %v, want %v", err, ErrTableNotFound)
	}
	if err := service.Leave(context.Background(), "invalid", guest); !errors.Is(err, ErrTableNotFound) {
		t.Fatalf("Leave() error = %v, want %v", err, ErrTableNotFound)
	}
}

func testTableService(t *testing.T, repository Repository) *Service {
	t.Helper()
	randomBytes := make([]byte, 1024)
	for _index := range randomBytes {
		randomBytes[_index] = byte(_index)
	}
	service, err := NewService(repository, []byte(strings.Repeat("table-pepper", 3)), bytes.NewReader(randomBytes), func() time.Time {
		return testJoinedAt
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}
