// ─── Витрина реестра сетей ──────────────────────────────────────────────────

package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"backend/config"
	"backend/repository"
	"backend/services"

	"github.com/gin-gonic/gin"
)

// dashboardQuarters разбирает выбранные кварталы. Пустой выбор — весь год:
// витрина не должна открываться пустой. Набор, а не диапазон, потому что
// сравнивают и несмежные кварталы — например, Q1 с Q3.
func dashboardQuarters(c *gin.Context) ([]int, bool) {
	raw := c.QueryArray("quarter")
	if len(raw) == 0 {
		return nil, true
	}
	quarters := make([]int, 0, len(raw))
	for _, value := range raw {
		quarter, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || quarter < 1 || quarter > 4 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный квартал"})
			return nil, false
		}
		quarters = append(quarters, quarter)
	}
	return quarters, true
}

// dashboardNetworkIDs — необязательный фильтр по конкретным сетям.
// Нечисловые значения игнорируются: фильтр сужает область, а не открывает её.
func dashboardNetworkIDs(c *gin.Context) []int {
	raw := c.QueryArray("network_id")
	ids := make([]int, 0, len(raw))
	for _, value := range raw {
		if id, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

// GetNetworkDashboard — сводная витрина реестра по всем доступным сетям.
//
// Область видимости та же, что у списка сетей: КАМ ведёт реестр за себя и на
// витрине видит только свои сети. Поэтому закрепление не дополняет фильтр
// запроса, а подменяет его — иначе КАМ впервые увидел бы чужой портфель.
func GetNetworkDashboard(c *gin.Context) {
	ownKAM, ok := networkOwnKAM(c)
	if !ok {
		return
	}
	year, ok := planYear(c)
	if !ok {
		return
	}
	quarters, ok := dashboardQuarters(c)
	if !ok {
		return
	}

	requestedKAMs := c.QueryArray("kam")
	if ownKAM != "" {
		requestedKAMs = nil
	}

	dashboard, err := services.BuildNetworkDashboard(repository.NetworkDashboardFilter{
		Year:       year,
		Quarters:   quarters,
		OwnKAM:     ownKAM,
		KAMs:       requestedKAMs,
		NetworkIDs: dashboardNetworkIDs(c),
	})
	if err != nil {
		config.Logger.Error("network_dashboard_failed", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить витрину реестра"})
		return
	}
	c.JSON(http.StatusOK, dashboard)
}
