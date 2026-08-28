package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"backend/config"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

// Область видимости на входе в обработчик. Здесь решается, чьи промо и чьи сети
// увидит вошедший, и здесь же выбирается код отказа, поэтому проверяем не
// только значения, но и ответ клиенту.

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

func contextForUser(username, role string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set("username", username)
	c.Set("role", role)
	return c, recorder
}

func TestPromoVisibilityScopeReturnsScope(t *testing.T) {
	withTestLogger(t)
	mock := withMockDB(t)
	mock.ExpectQuery("SELECT kam FROM").
		WithArgs("head.kam", "head.kam").
		WillReturnRows(sqlmock.NewRows([]string{"kam"}).AddRow("Ершов Максим"))

	c, _ := contextForUser("head.kam", "kam")
	scope, ok := promoVisibilityScope(c)
	if !ok || len(scope) != 1 || scope[0] != "Ершов Максим" {
		t.Fatalf("scope = %#v ok = %v", scope, ok)
	}
}

// Незакреплённая учётная запись получает 403 с прямым объяснением, а не пустую
// таблицу: молчаливый пустой экран читался бы как «промо нет».
func TestPromoVisibilityScopeRejectsUnlinkedKAM(t *testing.T) {
	withTestLogger(t)
	mock := withMockDB(t)
	mock.ExpectQuery("SELECT kam FROM").
		WithArgs("new.kam", "new.kam").
		WillReturnRows(sqlmock.NewRows([]string{"kam"}))

	c, recorder := contextForUser("new.kam", "kam")
	if _, ok := promoVisibilityScope(c); ok {
		t.Fatal("незакреплённая учётная запись не должна получать область")
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("код ответа = %d, ожидался 403", recorder.Code)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "не привязана к КАМу") {
		t.Fatalf("тело ответа = %s", body)
	}
}

// Сбой базы — не повод открыть доступ: обработчик отвечает 500 и не продолжает.
func TestPromoVisibilityScopeFailsClosedOnDBError(t *testing.T) {
	withTestLogger(t)
	mock := withMockDB(t)
	mock.ExpectQuery("SELECT kam FROM").
		WithArgs("kam.ershov", "kam.ershov").
		WillReturnError(errors.New("соединение потеряно"))

	c, recorder := contextForUser("kam.ershov", "kam")
	if _, ok := promoVisibilityScope(c); ok {
		t.Fatal("при ошибке БД область выдавать нельзя")
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("код ответа = %d, ожидался 500", recorder.Code)
	}
}

func TestNetworkOwnKAMRejectsUnlinkedKAM(t *testing.T) {
	withTestLogger(t)
	mock := withMockDB(t)
	mock.ExpectQuery("SELECT ISNULL").
		WithArgs("new.kam").
		WillReturnRows(sqlmock.NewRows([]string{"kam"}).AddRow(""))

	c, recorder := contextForUser("new.kam", "kam")
	if _, ok := networkOwnKAM(c); ok {
		t.Fatal("реестр не должен открываться без закрепления")
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("код ответа = %d, ожидался 403", recorder.Code)
	}
}

func TestNetworkWriteKAMKeepsOwnerWithinAssignment(t *testing.T) {
	withTestLogger(t)
	mock := withMockDB(t)
	mock.ExpectQuery("SELECT ISNULL").
		WithArgs("kam.ershov").
		WillReturnRows(sqlmock.NewRows([]string{"kam"}).AddRow("Ершов Максим"))

	c, _ := contextForUser("kam.ershov", "kam")
	// Поле не прислано: владелец берётся из текущего значения сети.
	kam, ok := networkWriteKAM(c, nil, "Ершов Максим")
	if !ok || kam != "Ершов Максим" {
		t.Fatalf("networkWriteKAM() = (%q, %v)", kam, ok)
	}
}

func TestNetworkWriteKAMRejectsForeignOwner(t *testing.T) {
	withTestLogger(t)
	mock := withMockDB(t)
	mock.ExpectQuery("SELECT ISNULL").
		WithArgs("kam.ershov").
		WillReturnRows(sqlmock.NewRows([]string{"kam"}).AddRow("Ершов Максим"))

	foreign := "Белов Андрей"
	c, recorder := contextForUser("kam.ershov", "kam")
	if _, ok := networkWriteKAM(c, &foreign, "Ершов Максим"); ok {
		t.Fatal("перенос сети на чужое имя должен отклоняться")
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("код ответа = %d, ожидался 403", recorder.Code)
	}
}

// Администратор запросом к базе не ограничивается и может передать любое имя.
func TestNetworkWriteKAMAllowsAdminToReassign(t *testing.T) {
	withTestLogger(t)
	withMockDB(t)

	target := "Белов Андрей"
	c, _ := contextForUser("demo_admin", "admin")
	kam, ok := networkWriteKAM(c, &target, "Ершов Максим")
	if !ok || kam != "Белов Андрей" {
		t.Fatalf("networkWriteKAM() = (%q, %v)", kam, ok)
	}
}
