package handlers

import "testing"

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
