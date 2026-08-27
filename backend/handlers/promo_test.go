package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAgreementNumberForRole(t *testing.T) {
	tests := []struct {
		name          string
		role          string
		requestedRole string
		want          int
		wantOK        bool
	}{
		{name: "agreement1 keeps own level", role: "agreement1", requestedRole: "agreement2", want: 1, wantOK: true},
		{name: "agreement2 keeps own level", role: "agreement2", requestedRole: "agreement1", want: 2, wantOK: true},
		{name: "admin selects agreement1", role: "admin", requestedRole: "agreement1", want: 1, wantOK: true},
		{name: "admin selects agreement2", role: "admin", requestedRole: "agreement2", want: 2, wantOK: true},
		{name: "admin must select a level", role: "admin", wantOK: false},
		{name: "unknown role is rejected", role: "viewer", requestedRole: "agreement1", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := agreementNumberForRole(tt.role, tt.requestedRole)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("agreementNumberForRole(%q, %q) = (%d, %v), want (%d, %v)", tt.role, tt.requestedRole, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestPromoWriteAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		scope    []string
		kam      string
		want     bool
		wantCode int
	}{
		{name: "без области пишет любого КАМа", scope: nil, kam: "Ершов Максим", want: true},
		{name: "свой КАМ в области", scope: []string{"Ершов Максим"}, kam: "Ершов Максим", want: true},
		{name: "подчинённый в области", scope: []string{"Ершов Максим", "Белов Андрей"}, kam: "Белов Андрей", want: true},
		{name: "чужой КАМ отклоняется", scope: []string{"Ершов Максим"}, kam: "Белов Андрей", want: false, wantCode: http.StatusForbidden},
		{name: "пустой КАМ вне области отклоняется", scope: []string{"Ершов Максим"}, kam: "", want: false, wantCode: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)

			if got := promoWriteAllowed(c, tt.scope, tt.kam); got != tt.want {
				t.Fatalf("promoWriteAllowed(%v, %q) = %v, want %v", tt.scope, tt.kam, got, tt.want)
			}
			if tt.want {
				return
			}
			if recorder.Code != tt.wantCode {
				t.Fatalf("код ответа = %d, want %d", recorder.Code, tt.wantCode)
			}
		})
	}
}
