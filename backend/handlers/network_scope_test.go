package handlers

import (
	"errors"
	"testing"
)

func TestNetworkKAMForWrite(t *testing.T) {
	own := "Ершов Максим"
	other := "Белов Андрей"
	empty := ""

	tests := []struct {
		name      string
		requested *string
		current   string
		ownKAM    string
		want      string
		wantErr   error
	}{
		{
			name:    "PATCH без поля сохраняет владельца",
			current: own, ownKAM: own, want: own,
		},
		{
			name:      "PATCH без поля не обнуляет владельца у администратора",
			requested: nil, current: other, ownKAM: "", want: other,
		},
		{
			name:      "администратор переназначает владельца",
			requested: &other, current: own, ownKAM: "", want: other,
		},
		{
			name:      "администратор обнуляет владельца явной пустой строкой",
			requested: &empty, current: own, ownKAM: "", want: "",
		},
		{
			name:      "КАМ пишет своё имя",
			requested: &own, current: own, ownKAM: own, want: own,
		},
		{
			name:      "пустое поле у КАМа заполняется его закреплением",
			requested: &empty, current: own, ownKAM: own, want: own,
		},
		{
			name:      "новая сеть КАМа заводится на него самого",
			requested: &empty, current: "", ownKAM: own, want: own,
		},
		{
			name:      "КАМ не заводит сеть на чужое имя",
			requested: &other, current: "", ownKAM: own, wantErr: errNetworkKAMOutOfScope,
		},
		{
			name:      "КАМ не переназначает свою сеть коллеге",
			requested: &other, current: own, ownKAM: own, wantErr: errNetworkKAMOutOfScope,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := networkKAMForWrite(tt.requested, tt.current, tt.ownKAM)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ошибка = %v, ожидалась %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && got != tt.want {
				t.Fatalf("владелец = %q, ожидался %q", got, tt.want)
			}
		})
	}
}
