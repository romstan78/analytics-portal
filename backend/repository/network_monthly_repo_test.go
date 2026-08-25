package repository

import (
	"errors"
	"testing"
	"time"
)

func TestClearNetworkForecastMonthRejectsClosedPeriodBeforeWriting(t *testing.T) {
	_, err := ClearNetworkForecastMonth(ClearNetworkForecastInput{
		NetworkID: 1,
		Year:      time.Now().Year() - 1,
		Month:     12,
		Scope:     "all",
	})
	if !errors.Is(err, ErrNetworkClosedMonth) {
		t.Fatalf("ошибка = %v, ожидалась ErrNetworkClosedMonth", err)
	}
}

func TestClearNetworkForecastMonthRejectsUnknownScopeBeforeWriting(t *testing.T) {
	_, err := ClearNetworkForecastMonth(ClearNetworkForecastInput{
		NetworkID: 1,
		Year:      time.Now().Year() + 1,
		Month:     1,
		Scope:     "unknown",
	})
	if err == nil {
		t.Fatal("ожидалась ошибка неизвестной области очистки")
	}
}
