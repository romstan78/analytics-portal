package repository

import (
	"testing"
	"time"
)

func testPolicy() LoginLockoutPolicy {
	return LoginLockoutPolicy{Threshold: 3, Window: 15 * time.Minute, Lockout: 15 * time.Minute}
}

func TestNextAttemptStateCountsSeries(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	policy := testPolicy()

	first := NextAttemptState(LoginAttemptState{}, now, policy)
	if first.FailedCount != 1 || first.Locked(now) {
		t.Fatalf("первая неудача = %+v, блокировать рано", first)
	}

	second := NextAttemptState(first, now.Add(time.Minute), policy)
	if second.FailedCount != 2 || second.Locked(now) {
		t.Fatalf("вторая неудача = %+v", second)
	}

	third := NextAttemptState(second, now.Add(2*time.Minute), policy)
	if !third.Locked(now.Add(2 * time.Minute)) {
		t.Fatalf("на пороге вход должен закрыться: %+v", third)
	}
	// Счётчик обнуляется: следующая серия считается заново, а не блокирует
	// учётную запись с первой же опечатки после разблокировки.
	if third.FailedCount != 0 {
		t.Fatalf("счётчик после блокировки = %d, ожидался 0", third.FailedCount)
	}
}

// Редкие опечатки не должны складываться: между ними проходит больше окна.
func TestNextAttemptStateForgetsOldFailures(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	policy := testPolicy()

	old := LoginAttemptState{FailedCount: 2, LastFailedAt: now.Add(-time.Hour)}
	next := NextAttemptState(old, now, policy)
	if next.FailedCount != 1 {
		t.Fatalf("счётчик = %d, старые неудачи должны забываться", next.FailedCount)
	}
	if next.Locked(now) {
		t.Fatal("одна свежая неудача не может закрыть вход")
	}
}

// LOGIN_LOCKOUT_MINUTES=0 отключает блокировку целиком — остаётся только
// лимит по адресу.
func TestNextAttemptStateRespectsDisabledPolicy(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	policy := LoginLockoutPolicy{Threshold: 3, Window: 15 * time.Minute, Lockout: 0}

	state := LoginAttemptState{}
	for i := 0; i < 10; i++ {
		state = NextAttemptState(state, now.Add(time.Duration(i)*time.Second), policy)
	}
	if state.Locked(now) {
		t.Fatalf("при выключенной блокировке вход закрывать нельзя: %+v", state)
	}
	if state.FailedCount != 10 {
		t.Fatalf("счётчик = %d, ожидалось 10", state.FailedCount)
	}
}

func TestLoginAttemptStateLockedBoundaries(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)

	if (LoginAttemptState{}).Locked(now) {
		t.Fatal("пустое состояние не блокирует")
	}
	past := LoginAttemptState{LockedUntil: now.Add(-time.Second)}
	if past.Locked(now) {
		t.Fatal("истёкшая блокировка больше не действует")
	}
	future := LoginAttemptState{LockedUntil: now.Add(time.Minute)}
	if !future.Locked(now) {
		t.Fatal("действующая блокировка должна закрывать вход")
	}
}

func TestLoginLockoutPolicyEnabled(t *testing.T) {
	if (LoginLockoutPolicy{Threshold: 3, Lockout: time.Minute}).Enabled() != true {
		t.Fatal("политика с порогом и сроком включена")
	}
	if (LoginLockoutPolicy{Threshold: 0, Lockout: time.Minute}).Enabled() {
		t.Fatal("нулевой порог выключает блокировку")
	}
	if (LoginLockoutPolicy{Threshold: 3, Lockout: 0}).Enabled() {
		t.Fatal("нулевой срок выключает блокировку")
	}
}
