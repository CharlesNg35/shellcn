package server

import (
	"errors"
	"testing"

	"github.com/charlesng35/shellcn/internal/service"
)

func TestEnrollmentRejectionClassification(t *testing.T) {
	if !isEnrollmentRejection(service.ErrEnrollmentInvalid) {
		t.Fatal("invalid enrollment must be a permanent rejection")
	}
	if isEnrollmentRejection(service.ErrNoAgentSupport) {
		t.Fatal("gateway configuration errors must be retryable")
	}
	if isEnrollmentRejection(errors.New("store unavailable")) {
		t.Fatal("gateway failures must be retryable")
	}
}
