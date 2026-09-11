// ─── Реестр сетей: бюджет OPEX по контракту ─────────────────────────────────

package handlers

import (
	"math"
	"net/http"
	"strings"

	"backend/models"
	"backend/repository"
	"backend/services"

	"github.com/gin-gonic/gin"
)

// opexAmountLimit — потолок одной квартальной суммы. Колонка DECIMAL(18,2)
// выдержит больше, но ввод в триллион рублей — это опечатка, и сказать о ней
// внятным 400 лучше, чем отдать 500 из-за переполнения.
const opexAmountLimit = 1e12

// loadNetworkOpex собирает вкладку за год. Чтение и ответ на сохранение ходят
// одним путём: две сборки одного экрана со временем расходятся.
func loadNetworkOpex(networkID, year int) (models.NetworkOpexResponse, error) {
	network, err := repository.GetNetworkByID(networkID)
	if err != nil {
		return models.NetworkOpexResponse{}, err
	}
	brands, err := repository.NetworkPlanBrands(networkID, year)
	if err != nil {
		return models.NetworkOpexResponse{}, err
	}
	rows, err := repository.GetNetworkOpexBudgets(networkID, year)
	if err != nil {
		return models.NetworkOpexResponse{}, err
	}
	return services.BuildNetworkOpexResponse(network, year, brands, rows), nil
}

func GetNetworkOpex(c *gin.Context) {
	id, ok := networkIDParam(c)
	if !ok {
		return
	}
	year, ok := planYear(c)
	if !ok {
		return
	}
	response, err := loadNetworkOpex(id, year)
	if err != nil {
		respondNetworkError(c, err, "network_opex_failed")
		return
	}
	c.JSON(http.StatusOK, response)
}

type saveOpexInput struct {
	Year int                           `json:"year"`
	Rows []repository.NetworkOpexInput `json:"rows"`
}

// SaveNetworkOpex сохраняет квартальные ячейки бюджета.
//
// Приходят кварталы — то, что согласует КАМ; в базу уходят месяцы. Раскладку
// считает services, а не форма: вторая копия правила распределения на клиенте
// со временем разошлась бы с этой, и сумма месяцев перестала бы сходиться
// с введённым кварталом.
func SaveNetworkOpex(c *gin.Context) {
	id, ok := networkIDParam(c)
	if !ok {
		return
	}
	var input saveOpexInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.Year < 2000 || input.Year > 2100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный год"})
		return
	}

	network, err := repository.GetNetworkByID(id)
	if err != nil {
		respondNetworkError(c, err, "network_opex_save_fetch_failed")
		return
	}
	persistedPeriods, err := repository.GetNetworkPeriods(id, input.Year)
	if err != nil {
		respondNetworkError(c, err, "network_opex_periods_failed")
		return
	}
	// НДС квартала — тем же правилом, что и всюду в реестре: незаведённый
	// квартал берёт ставку из карточки сети. Карта строится один раз: ячеек
	// в запросе бывают сотни, а кварталов всегда четыре.
	vatByQuarter := map[int]models.NetworkPeriod{}
	for _, period := range services.NetworkPeriodsWithDefaults(network, input.Year, persistedPeriods) {
		vatByQuarter[period.Quarter] = period
	}

	cells := make([]repository.NetworkOpexCellWrite, 0, len(input.Rows))
	for _, row := range input.Rows {
		article := strings.TrimSpace(row.Article)
		if !services.IsNetworkOpexArticle(article) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Неизвестная статья бюджета OPEX: " + article})
			return
		}
		if row.Quarter < 1 || row.Quarter > 4 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный квартал"})
			return
		}
		brand := strings.TrimSpace(row.BrandAS)
		if brand == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Бренд бюджета OPEX не указан"})
			return
		}
		if row.AmountRub != nil && math.Abs(*row.AmountRub) > opexAmountLimit {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Сумма бюджета OPEX выходит за допустимые пределы"})
			return
		}

		cell := repository.NetworkOpexCellWrite{
			Quarter:   row.Quarter,
			BrandAS:   brand,
			Article:   article,
			UpdatedAt: strings.TrimSpace(row.UpdatedAt),
		}
		if row.AmountRub != nil {
			vat := vatByQuarter[row.Quarter]
			for _, month := range services.NetworkOpexMonthlyRows(
				row.Quarter, *row.AmountRub, vat.VATIncluded, vat.VATRate,
			) {
				cell.Months = append(cell.Months, repository.NetworkOpexMonthWrite{
					Month: month.Month, AmountRub: month.AmountRub, AmountNet: month.AmountNet,
				})
			}
		}
		cells = append(cells, cell)
	}

	username, _ := currentUser(c)
	if err := repository.SaveNetworkOpexBudgets(repository.SaveNetworkOpexInput{
		NetworkID: id, Year: input.Year, Cells: cells, UserName: username,
	}); err != nil {
		respondNetworkError(c, err, "network_opex_save_failed")
		return
	}

	response, err := loadNetworkOpex(id, input.Year)
	if err != nil {
		respondNetworkError(c, err, "network_opex_refetch_failed")
		return
	}
	_ = repository.InsertEntityAuditLog(
		"network_opex", id, username, "UPDATE",
		jsonString(map[string]interface{}{
			"year": input.Year, "cells": len(cells), "total_rub": response.Totals.AmountRub,
		}),
	)
	c.JSON(http.StatusOK, models.NetworkOpexSaveResponse{Message: "Saved", Data: response})
}
