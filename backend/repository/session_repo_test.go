package repository

import (
	"strings"
	"testing"
)

func TestHashRefreshTokenIsStableAndOpaque(t *testing.T) {
	const token = "eyJhbGciOiJIUzI1NiJ9.payload.signature"

	first := HashRefreshToken(token)
	second := HashRefreshToken(token)
	if first != second {
		t.Fatalf("хеш не детерминирован: %q != %q", first, second)
	}
	if len(first) != 64 {
		t.Fatalf("длина хеша = %d, ожидалось 64 (SHA-256 в hex)", len(first))
	}
	if strings.Contains(first, token) {
		t.Fatal("хеш не должен содержать сам токен")
	}
	if HashRefreshToken(token+"x") == first {
		t.Fatal("разные токены дали одинаковый хеш")
	}
}
