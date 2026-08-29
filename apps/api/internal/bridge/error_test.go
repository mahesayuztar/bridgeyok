package bridge

import "testing"

func TestDomainErrorError(t *testing.T) {
	t.Parallel()

	if got := (*DomainError)(nil).Error(); got != "" {
		t.Errorf("nil DomainError.Error() = %q, want empty", got)
	}
	domainError := reject(ErrorIllegalCall, "illegal")
	if domainError.Code != ErrorIllegalCall || domainError.Error() != "illegal" {
		t.Errorf("reject() = %+v", domainError)
	}
}
