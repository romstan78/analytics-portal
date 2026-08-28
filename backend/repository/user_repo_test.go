package repository

import (
	"errors"
	"testing"
)

func TestKAMLinkRequired(t *testing.T) {
	tests := []struct {
		name    string
		role    string
		linked  bool
		wantErr error
	}{
		{name: "закреплённый КАМ работает", role: "kam", linked: true},
		{name: "КАМ без закрепления не получает доступ", role: "kam", wantErr: ErrKAMNotLinked},
		{name: "администратор без закрепления не ограничен", role: "admin"},
		{name: "унаследованный согласующий первой ступени не ограничен", role: "agreement1"},
		{name: "унаследованный согласующий второй ступени не ограничен", role: "agreement2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := kamLinkRequired(tt.role, tt.linked); !errors.Is(err, tt.wantErr) {
				t.Fatalf("kamLinkRequired(%q, %v) = %v, ожидалось %v", tt.role, tt.linked, err, tt.wantErr)
			}
		})
	}
}
