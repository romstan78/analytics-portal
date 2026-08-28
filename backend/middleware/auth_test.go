package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"backend/config"

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

// ─── AuthRequired ───────────────────────────────────────────────────────────
//
// Через этот middleware проходит весь /api, и именно он кладёт в контекст
// username и role, по которым дальше считается область видимости. До сих пор он
// не был покрыт вовсе.

func authRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", AuthRequired(), func(c *gin.Context) {
		username, _ := c.Get("username")
		role, _ := c.Get("role")
		c.JSON(http.StatusOK, gin.H{"username": username, "role": role})
	})
	return r
}

func requestWithAuth(t *testing.T, header string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	w := httptest.NewRecorder()
	authRouter().ServeHTTP(w, req)
	return w
}

func TestAuthRequiredRejectsMissingAndMalformedHeader(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{name: "заголовка нет", header: ""},
		{name: "без схемы", header: "sometoken"},
		{name: "чужая схема", header: "Basic dXNlcjpwYXNz"},
		{name: "строчная схема — регистр важен", header: "bearer sometoken"},
		{name: "схема без токена", header: "Bearer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code := requestWithAuth(t, tt.header).Code; code != http.StatusUnauthorized {
				t.Fatalf("код ответа = %d, ожидался 401", code)
			}
		})
	}
}

func TestAuthRequiredRejectsInvalidToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "secret-not-shorter-than-thirty-two-chars")
	if err := config.InitAuth(); err != nil {
		t.Fatalf("InitAuth() = %v", err)
	}
	// Подпись чужим секретом: без проверки подписи такой токен пускал бы кого
	// угодно с самодельным набором ролей.
	for _, token := range []string{"явно.не.jwt", "Bearer", strings.Repeat("a", 40)} {
		if code := requestWithAuth(t, "Bearer "+token).Code; code != http.StatusUnauthorized {
			t.Fatalf("токен %q дал код %d, ожидался 401", token, code)
		}
	}
}

func TestAuthRequiredPassesUsernameAndRole(t *testing.T) {
	t.Setenv("JWT_SECRET", "secret-not-shorter-than-thirty-two-chars")
	if err := config.InitAuth(); err != nil {
		t.Fatalf("InitAuth() = %v", err)
	}
	token, err := config.GenerateAccessToken("kam.ershov", "kam")
	if err != nil {
		t.Fatalf("GenerateAccessToken() = %v", err)
	}

	w := requestWithAuth(t, "Bearer "+token)
	if w.Code != http.StatusOK {
		t.Fatalf("код ответа = %d, ожидался 200: %s", w.Code, w.Body.String())
	}
	// Область видимости считается по этим двум значениям, поэтому проверяем
	// не только пропуск, но и то, что именно попало в контекст.
	var body struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("разбор ответа: %v", err)
	}
	if body.Username != "kam.ershov" || body.Role != "kam" {
		t.Fatalf("контекст = %+v, ожидались kam.ershov/kam", body)
	}
}

// Refresh-токен не должен пускать в API: у него другой тип, и принимать его
// вместо access-токена значило бы продлевать доступ в обход срока жизни.
func TestAuthRequiredRejectsRefreshToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "secret-not-shorter-than-thirty-two-chars")
	if err := config.InitAuth(); err != nil {
		t.Fatalf("InitAuth() = %v", err)
	}
	refresh, err := config.GenerateRefreshToken("kam.ershov", "kam")
	if err != nil {
		t.Fatalf("GenerateRefreshToken() = %v", err)
	}
	if code := requestWithAuth(t, "Bearer "+refresh).Code; code != http.StatusUnauthorized {
		t.Fatalf("refresh-токен дал код %d, ожидался 401", code)
	}
}
