package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func roleRouter(role string, allowed ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", func(c *gin.Context) {
		c.Set("role", role)
		c.Next()
	}, RoleRequired(allowed...), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	return r
}

func TestRoleRequiredAllowsConfiguredRole(t *testing.T) {
	w := httptest.NewRecorder()
	roleRouter("agreement1", "agreement1", "agreement2").ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/protected", nil))
	if w.Code != http.StatusNoContent {
		t.Fatalf("ожидался 204, получен %d", w.Code)
	}
}

func TestRoleRequiredRejectsOtherRole(t *testing.T) {
	w := httptest.NewRecorder()
	roleRouter("admin", "agreement1", "agreement2").ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/protected", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("ожидался 403, получен %d", w.Code)
	}
}
