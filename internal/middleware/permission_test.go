package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/permissions"
)

func TestRequirePermissionWithoutAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Build a checker with a nil DB (use a minimal stub pattern if needed)
	// Here, we'll create a checker using an in-memory sqlite DB via permissions tests helpers is not exported.
	// Instead, construct a checker with a fake DB by passing nil will error; so we skip checker invocation
	// by hitting the early 401 branch due to missing userID in context.
	r := gin.New()
	// Use a dummy handler that should never be called
	r.GET("/secure", RequirePermission(&permissions.Checker{}, "user.view"), func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/secure", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
