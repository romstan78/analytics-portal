package handlers

import (
	"testing"
	"time"

	"backend/repository"

	"golang.org/x/crypto/bcrypt"
)

// Ветка «пользователь не найден» обязана сравнивать пароль с фиктивным хешем:
// без него ответ приходил мгновенно, и по времени отклика перебирались имена
// учётных записей, не зная ни одного пароля.
func TestPasswordHashForUnknownUserReturnsUsableHash(t *testing.T) {
	hash := passwordHashFor(nil)
	if len(hash) == 0 {
		t.Fatal("для ненайденного пользователя не с чем сравнивать пароль")
	}
	cost, err := bcrypt.Cost(hash)
	if err != nil {
		t.Fatalf("фиктивный хеш не читается bcrypt: %v", err)
	}
	// Стоимость обязана совпадать с настоящей, иначе сравнение окажется
	// быстрее и разница во времени вернётся.
	if cost != bcryptCost {
		t.Fatalf("стоимость фиктивного хеша = %d, ожидалась %d", cost, bcryptCost)
	}
	if bcrypt.CompareHashAndPassword(hash, []byte("любой пароль")) == nil {
		t.Fatal("фиктивный хеш не должен совпадать ни с одним паролем")
	}
}

func TestPasswordHashForKnownUserReturnsStoredHash(t *testing.T) {
	stored, err := bcrypt.GenerateFromPassword([]byte("пароль"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("подготовка хеша: %v", err)
	}
	user := &repository.UserRecord{Username: "kam.ershov", PasswordHash: string(stored)}
	if got := string(passwordHashFor(user)); got != string(stored) {
		t.Fatalf("для найденного пользователя взят чужой хеш: %q", got)
	}
}

// Проверка стоимости через время: сравнение с фиктивным хешем должно занимать
// столько же, сколько с настоящим той же стоимости. Порог намеренно грубый —
// тест ловит подмену стоимости на минимальную, а не измеряет производительность.
func TestDummyHashComparisonIsNotInstant(t *testing.T) {
	if testing.Short() {
		t.Skip("замер времени пропускается в коротком режиме")
	}
	hash := passwordHashFor(nil)

	start := time.Now()
	_ = bcrypt.CompareHashAndPassword(hash, []byte("подбор"))
	elapsed := time.Since(start)

	if elapsed < 10*time.Millisecond {
		t.Fatalf("сравнение заняло %v — стоимость хеша слишком низкая, разница во времени выдаст логин", elapsed)
	}
}
