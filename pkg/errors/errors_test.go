package errors

import (
	stdErrors "errors"
	"testing"
)

func TestErrorIncludesInternal(t *testing.T) {
	internal := stdErrors.New("boom")
	err := Wrap(internal, "failed")

	if err.Error() != "failed: boom" {
		t.Fatalf("unexpected error string: %s", err.Error())
	}
}

func TestWithInternalCopies(t *testing.T) {
	base := New("TEST", "test", 400)
	with := base.WithInternal(stdErrors.New("oops"))

	if with == base {
		t.Fatal("expected WithInternal to return a copy")
	}

	if base.Internal != nil {
		t.Fatal("expected original error to remain unchanged")
	}

	if with.Internal == nil {
		t.Fatal("expected internal error to be set")
	}
}

func TestFromError(t *testing.T) {
	appErr := ErrNotFound
	if out := FromError(appErr); out != appErr {
		t.Fatal("expected FromError to return the same AppError instance")
	}

	raw := stdErrors.New("raw")
	out := FromError(raw)
	if out.Code != ErrInternalServer.Code {
		t.Fatalf("expected internal server code, got %s", out.Code)
	}
	if out.Internal == nil {
		t.Fatal("expected internal error to be attached")
	}
}

func TestNewBadRequest(t *testing.T) {
	err := NewBadRequest("invalid payload")
	if err.Code != ErrBadRequest.Code {
		t.Fatalf("expected %s, got %s", ErrBadRequest.Code, err.Code)
	}
	if err.Message != "invalid payload" {
		t.Fatalf("unexpected message: %s", err.Message)
	}
	if err.StatusCode != ErrBadRequest.StatusCode {
		t.Fatalf("unexpected status: %d", err.StatusCode)
	}
}
