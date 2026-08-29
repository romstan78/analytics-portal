package repository

import (
	"errors"
	"testing"
)

func TestIsDuplicateDictionaryError(t *testing.T) {
	for _, message := range []string{
		"Violation of UNIQUE KEY constraint",
		"Cannot insert duplicate key row (2601)",
		"2627: duplicate value",
	} {
		if !IsDuplicateDictionaryError(errors.New(message)) {
			t.Fatalf("ошибка %q не распознана как дубль", message)
		}
	}
	if IsDuplicateDictionaryError(errors.New("connection reset")) {
		t.Fatal("сетевая ошибка распознана как дубль")
	}
}
