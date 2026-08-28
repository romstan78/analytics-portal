package repository

import (
	"errors"
	"testing"
	"time"

	"backend/config"

	"github.com/DATA-DOG/go-sqlmock"
)

// Проверки доступа, которые до сих пор не были покрыты: они ходят в БД, и без
// подмены соединения их нельзя было проверить. Именно здесь решается, чьи промо
// и чьи сети увидит вошедший, поэтому цена ошибки тут выше, чем в расчётах.

// withMockDB подменяет глобальное соединение на время теста.
func withMockDB(t *testing.T) sqlmock.Sqlmock {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	previous := config.DB
	config.DB = db
	t.Cleanup(func() {
		config.DB = previous
		_ = db.Close()
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("невыполненные ожидания: %v", err)
		}
	})
	return mock
}

func TestGetPromoVisibilityScopeAdminIsUnrestricted(t *testing.T) {
	// Администратору запрос вообще не нужен: ожиданий не задаём, и лишний
	// поход в базу провалит тест.
	withMockDB(t)

	scope, err := GetPromoVisibilityScope("demo_admin", "admin")
	if err != nil || scope != nil {
		t.Fatalf("scope = %#v err = %v, администратор не ограничивается", scope, err)
	}
}

func TestGetPromoVisibilityScopeCollectsOwnAndSubordinates(t *testing.T) {
	mock := withMockDB(t)
	mock.ExpectQuery("SELECT kam FROM").
		WithArgs("head.kam", "head.kam").
		WillReturnRows(sqlmock.NewRows([]string{"kam"}).
			AddRow("Ершов Максим").
			AddRow("Жукова Ольга"))

	scope, err := GetPromoVisibilityScope("head.kam", "kam")
	if err != nil {
		t.Fatalf("GetPromoVisibilityScope() = %v", err)
	}
	// Руководитель видит и свой портфель, и портфели подчинённых.
	if len(scope) != 2 || scope[0] != "Ершов Максим" || scope[1] != "Жукова Ольга" {
		t.Fatalf("scope = %#v", scope)
	}
}

// Роль kam без единой строки области — незаполненное закрепление, а не право
// видеть всю компанию.
func TestGetPromoVisibilityScopeFailsClosedForUnlinkedKAM(t *testing.T) {
	mock := withMockDB(t)
	mock.ExpectQuery("SELECT kam FROM").
		WithArgs("new.kam", "new.kam").
		WillReturnRows(sqlmock.NewRows([]string{"kam"}))

	scope, err := GetPromoVisibilityScope("new.kam", "kam")
	if !errors.Is(err, ErrKAMNotLinked) {
		t.Fatalf("err = %v, ожидалась ErrKAMNotLinked", err)
	}
	if scope != nil {
		t.Fatalf("scope = %#v, при отказе область должна быть пустой", scope)
	}
}

// Унаследованный согласующий без области работает как раньше — без ограничения.
func TestGetPromoVisibilityScopeKeepsLegacyApproverOpen(t *testing.T) {
	mock := withMockDB(t)
	mock.ExpectQuery("SELECT kam FROM").
		WithArgs("agreement.user", "agreement.user").
		WillReturnRows(sqlmock.NewRows([]string{"kam"}))

	scope, err := GetPromoVisibilityScope("agreement.user", "agreement1")
	if err != nil || len(scope) != 0 {
		t.Fatalf("scope = %#v err = %v, прежнее поведение должно сохраниться", scope, err)
	}
}

func TestGetOwnKAMReturnsAssignment(t *testing.T) {
	mock := withMockDB(t)
	mock.ExpectQuery("SELECT ISNULL").
		WithArgs("kam.ershov").
		WillReturnRows(sqlmock.NewRows([]string{"kam"}).AddRow("Ершов Максим"))

	kam, err := GetOwnKAM("kam.ershov", "kam")
	if err != nil || kam != "Ершов Максим" {
		t.Fatalf("GetOwnKAM() = (%q, %v)", kam, err)
	}
}

func TestGetOwnKAMFailsClosedForUnlinkedKAM(t *testing.T) {
	mock := withMockDB(t)
	mock.ExpectQuery("SELECT ISNULL").
		WithArgs("new.kam").
		WillReturnRows(sqlmock.NewRows([]string{"kam"}).AddRow(""))

	if _, err := GetOwnKAM("new.kam", "kam"); !errors.Is(err, ErrKAMNotLinked) {
		t.Fatalf("err = %v, ожидалась ErrKAMNotLinked", err)
	}
}

// Удалённая учётная запись до запроса не доходит по deleted_at, и строк нет —
// для роли kam это тоже отказ, а не открытый доступ.
func TestGetOwnKAMFailsClosedWhenUserRowMissing(t *testing.T) {
	mock := withMockDB(t)
	mock.ExpectQuery("SELECT ISNULL").
		WithArgs("deleted.kam").
		WillReturnRows(sqlmock.NewRows([]string{"kam"}))

	if _, err := GetOwnKAM("deleted.kam", "kam"); !errors.Is(err, ErrKAMNotLinked) {
		t.Fatalf("err = %v, ожидалась ErrKAMNotLinked", err)
	}
}

func TestGetOwnKAMAdminSkipsQuery(t *testing.T) {
	withMockDB(t)

	kam, err := GetOwnKAM("demo_admin", "admin")
	if err != nil || kam != "" {
		t.Fatalf("GetOwnKAM() = (%q, %v), администратор не ограничивается", kam, err)
	}
}

func TestGetLoginAttemptStateReadsStoredCounters(t *testing.T) {
	mock := withMockDB(t)
	lastFailed := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	lockedUntil := lastFailed.Add(15 * time.Minute)
	mock.ExpectQuery("SELECT failed_count").
		WithArgs("kam.ershov").
		WillReturnRows(sqlmock.NewRows([]string{"failed_count", "last_failed_at", "locked_until"}).
			AddRow(3, lastFailed, lockedUntil))

	state, err := GetLoginAttemptState("kam.ershov")
	if err != nil {
		t.Fatalf("GetLoginAttemptState() = %v", err)
	}
	if state.FailedCount != 3 || !state.LastFailedAt.Equal(lastFailed) || !state.LockedUntil.Equal(lockedUntil) {
		t.Fatalf("state = %+v", state)
	}
	if !state.Locked(lastFailed.Add(time.Minute)) {
		t.Fatal("действующая блокировка должна закрывать вход")
	}
}

// Логина в таблице нет — неудач не было; это не ошибка.
func TestGetLoginAttemptStateEmptyForUnknownLogin(t *testing.T) {
	mock := withMockDB(t)
	mock.ExpectQuery("SELECT failed_count").
		WithArgs("нет-такого").
		WillReturnRows(sqlmock.NewRows([]string{"failed_count", "last_failed_at", "locked_until"}))

	state, err := GetLoginAttemptState("нет-такого")
	if err != nil {
		t.Fatalf("GetLoginAttemptState() = %v", err)
	}
	if state.FailedCount != 0 || state.Locked(time.Now()) {
		t.Fatalf("state = %+v, ожидалось пустое состояние", state)
	}
}

// Область видимости обязана попадать в счётный запрос: иначе «всего строк» под
// таблицей показывало бы размер чужой выборки.
func TestPromoRowsCountAppliesScope(t *testing.T) {
	mock := withMockDB(t)
	mock.ExpectQuery("SELECT COUNT").
		WithArgs("Жукова Ольга").
		WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(42))

	total, err := PromoRowsCount(PromoFilterParams{AllowedKAMs: []string{"Жукова Ольга"}}, nil)
	if err != nil || total != 42 {
		t.Fatalf("PromoRowsCount() = (%d, %v)", total, err)
	}
}
