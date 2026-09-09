package identity

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAccountInputAndAttemptLimits(t *testing.T) {
	now := time.Now().UTC()
	service, err := NewService(&memoryRepository{}, []byte(strings.Repeat("p", 32)), rand.Reader, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, username, password, displayName, avatar string }{
		{"short username", "ab", "long password", "Alice", "spade"},
		{"invalid username", "a@b", "long password", "Alice", "spade"},
		{"short password", "alice", "short", "Alice", "spade"},
		{"long password", "alice", strings.Repeat("p", 129), "Alice", "spade"},
		{"control name", "alice", "long password", "Alice\n", "spade"},
		{"unknown avatar", "alice", "long password", "Alice", "https://image.test"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := service.Register(context.Background(), testCase.username, testCase.password, testCase.displayName, testCase.avatar)
			if !errors.Is(err, ErrAccountInput) {
				t.Fatalf("Register() = %v", err)
			}
		})
	}
	now = now.Add(time.Minute)
	var wait sync.WaitGroup
	var mutex sync.Mutex
	accepted := 0
	for _attempt := 0; _attempt < 30; _attempt++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if _, err := service.Register(context.Background(), "same_user", "short", "Player", "spade"); !errors.Is(err, ErrAccountRate) {
				mutex.Lock()
				accepted++
				mutex.Unlock()
			}
		}()
	}
	wait.Wait()
	if accepted != 10 {
		t.Fatalf("concurrent attempts admitted %d, want 10", accepted)
	}
	now = now.Add(time.Minute)
	if _, err := service.Register(context.Background(), "same_user", "short", "Player", "spade"); errors.Is(err, ErrAccountRate) {
		t.Fatal("attempt window did not recover")
	}
}
