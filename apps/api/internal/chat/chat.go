package chat

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/rivo/uniseg"
)

var ErrInput = errors.New("invalid chat input")
var ErrAccess = errors.New("chat access denied")
var ErrConflict = errors.New("chat request identity reused")

const MaxGraphemes = 1000
const MaxBytes = 16000

type Message struct {
	MessageID       string    `json:"messageId"`
	Scope           string    `json:"scope"`
	ConversationID  string    `json:"conversationId"`
	SenderUserID    string    `json:"senderUserId"`
	Content         string    `json:"content"`
	ClientRequestID string    `json:"clientRequestId"`
	CreatedAt       time.Time `json:"createdAt"`
}

type Target struct {
	Scope string `json:"scope"`
	ID    string `json:"id"`
}

type Page struct {
	Messages   []Message `json:"messages"`
	NextCursor string    `json:"nextCursor,omitempty"`
}

type Repository interface {
	SendChat(context.Context, string, Target, string, string) (Message, bool, error)
	ChatHistory(context.Context, string, Target, string, int) (Page, error)
}

func Validate(target Target, requestID, content string) error {
	if target.Scope != "private" && target.Scope != "table" {
		return ErrInput
	}
	if _, err := uuid.Parse(target.ID); err != nil {
		return ErrInput
	}
	if len(requestID) < 8 || len(requestID) > 64 {
		return ErrInput
	}
	for _, character := range requestID {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '-' && character != '_' {
			return ErrInput
		}
	}
	if !utf8.ValidString(content) || len(content) > MaxBytes || strings.TrimSpace(content) == "" || strings.ContainsRune(content, 0) || uniseg.GraphemeClusterCount(content) > MaxGraphemes {
		return ErrInput
	}
	return nil
}
