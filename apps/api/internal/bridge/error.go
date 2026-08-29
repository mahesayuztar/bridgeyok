package bridge

// ErrorCode is a stable machine-readable domain rejection code.
type ErrorCode string

const (
	ErrorInvalidState          ErrorCode = "INVALID_STATE"
	ErrorInvalidCommand        ErrorCode = "INVALID_COMMAND"
	ErrorNotYourTurn           ErrorCode = "NOT_YOUR_TURN"
	ErrorIllegalCall           ErrorCode = "ILLEGAL_CALL"
	ErrorAuctionComplete       ErrorCode = "AUCTION_COMPLETE"
	ErrorCardNotHeld           ErrorCode = "CARD_NOT_HELD"
	ErrorMustFollowSuit        ErrorCode = "MUST_FOLLOW_SUIT"
	ErrorDeclarerControlsDummy ErrorCode = "DECLARER_CONTROLS_DUMMY"
	ErrorPlayComplete          ErrorCode = "PLAY_COMPLETE"
)

// DomainError describes a rejected command without exposing internal state.
type DomainError struct {
	Code    ErrorCode
	Message string
}

// Error returns the safe human-readable diagnostic for logs and tests.
func (domainError *DomainError) Error() string {
	if domainError == nil {
		return ""
	}
	return domainError.Message
}

func reject(code ErrorCode, message string) *DomainError {
	return &DomainError{Code: code, Message: message}
}
