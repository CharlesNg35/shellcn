package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	appErrors "github.com/charlesng35/shellcn/pkg/errors"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestSuccess(t *testing.T) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	payload := gin.H{"message": "ok"}
	Success(ctx, http.StatusCreated, payload)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d got %d", http.StatusCreated, rec.Code)
	}

	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Fatal("expected success flag to be true")
	}
	if resp.Error != nil {
		t.Fatal("expected no error information")
	}
}

func TestSuccessWithMeta(t *testing.T) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	meta := &Meta{Page: 1, PerPage: 10, Total: 20, TotalPages: 2}
	SuccessWithMeta(ctx, http.StatusOK, []string{"a", "b"}, meta)

	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Meta == nil || resp.Meta.Total != 20 {
		t.Fatal("expected metadata to be serialised")
	}
}

func TestErrorWithAppError(t *testing.T) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	Error(ctx, appErrors.ErrForbidden)

	if rec.Code != appErrors.ErrForbidden.StatusCode {
		t.Fatalf("expected status %d got %d", appErrors.ErrForbidden.StatusCode, rec.Code)
	}

	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Success {
		t.Fatal("expected success to be false")
	}
	if resp.Error == nil || resp.Error.Code != appErrors.ErrForbidden.Code {
		t.Fatal("expected forbidden error code in response")
	}
}

func TestErrorWithGenericError(t *testing.T) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	Error(ctx, errors.New("boom"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d got %d", http.StatusInternalServerError, rec.Code)
	}
}
