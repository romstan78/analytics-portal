package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend/config"
	"backend/models"
	"backend/repository"
	"backend/services"

	"github.com/gin-gonic/gin"
	mssql "github.com/microsoft/go-mssqldb"
)

func budgetFilter(c *gin.Context) (services.BudgetFilter, bool) {
	year, ok := planYear(c)
	if !ok {
		return services.BudgetFilter{}, false
	}
	f := services.BudgetFilter{Composition: c.DefaultQuery("composition", "expanded") != "collapsed", ExpandedBrands: c.QueryArray("expand"), Year: year, Source: c.DefaultQuery("src", "all"), Kind: c.DefaultQuery("kind", "all"), Gross: c.Query("vat") == "gross", NetworkTypes: c.QueryArray("types"), Base: c.DefaultQuery("base", "reg-contract")}
	if (f.Source != "all" && f.Source != "contract" && f.Source != "promo") || (f.Kind != "all" && f.Kind != "gtn" && f.Kind != "opex") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный источник или тип инвестиций"})
		return f, false
	}
	for key, target := range map[string]*[4]string{"to_sources": &f.ToSources, "investment_sources": &f.InvestmentSources} {
		parts := strings.Split(c.DefaultQuery(key, "forecast,forecast,forecast,forecast"), ",")
		if len(parts) != 4 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Выберите источник для каждого из четырёх кварталов"})
			return f, false
		}
		for i, part := range parts {
			if part != "plan" && part != "fact" && part != "forecast" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Источник: план, факт или прогноз"})
				return f, false
			}
			target[i] = part
		}
	}
	if base := f.Base; base != "reg-contract" && base != "reg-olap" && base != "ss" && base != "sswo" && base != "pure" && base != "omni" && base != "mp" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Некорректная база процента"})
		return f, false
	}
	return f, true
}

func GetBudget(c *gin.Context) {
	f, ok := budgetFilter(c)
	if !ok {
		return
	}
	r, err := services.BudgetView(f, c.DefaultQuery("version", "LIVE"), c.Query("compare"), c.Query("delta"))
	if err != nil {
		config.Logger.Error("budget_failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить бюджет"})
		return
	}
	services.BudgetEditable(r, f, c.GetString("role"))
	c.JSON(http.StatusOK, r)
}

func budgetError(c *gin.Context, err error) {
	code := http.StatusUnprocessableEntity
	if errors.Is(err, repository.ErrBudgetConflict) || errors.Is(err, repository.ErrBudgetVersionExists) {
		code = http.StatusConflict
	}
	if errors.Is(err, sql.ErrNoRows) {
		code = http.StatusNotFound
	}
	var dbError mssql.Error
	if errors.As(err, &dbError) {
		config.Logger.Error("budget_storage_failed", "error", err)
		c.JSON(500, gin.H{"error": "Не удалось сохранить или загрузить бюджет"})
		return
	}
	c.JSON(code, gin.H{"error": err.Error()})
}
func budgetID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		c.JSON(400, gin.H{"error": "Некорректная версия"})
		return 0, false
	}
	return id, true
}
func GetBudgetVersions(c *gin.Context) {
	year, ok := planYear(c)
	if !ok {
		return
	}
	v, err := repository.BudgetVersions(year)
	if err != nil {
		budgetError(c, err)
		return
	}
	c.JSON(200, v)
}
func CreateBudgetVersion(c *gin.Context) {
	var in struct {
		Year    int                  `json:"year"`
		Code    string               `json:"code"`
		Name    string               `json:"name"`
		Sources models.BudgetSources `json:"sources"`
	}
	if c.ShouldBindJSON(&in) != nil || in.Year < 2000 || in.Year > 2100 || (in.Code != "B" && in.Code != "F1" && in.Code != "F2" && in.Code != "F3" && in.Code != "LIVE") || !services.ValidBudgetSources(in.Sources) {
		c.JSON(422, gin.H{"error": "Укажите год, код версии и источники четырёх кварталов"})
		return
	}
	if in.Name == "" {
		in.Name = in.Code
	}
	v, err := repository.CreateBudgetVersion(in.Year, in.Code, in.Name, c.GetString("username"), in.Sources)
	if err != nil {
		budgetError(c, err)
		return
	}
	c.JSON(201, v)
}
func FreezeBudget(c *gin.Context) {
	id, ok := budgetID(c)
	if !ok {
		return
	}
	var in struct {
		UpdatedAt string `json:"updated_at"`
	}
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(400, gin.H{"error": "Некорректный запрос"})
		return
	}
	if err := services.FreezeBudget(id, in.UpdatedAt, c.GetString("username")); err != nil {
		budgetError(c, err)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func SaveBudgetCell(c *gin.Context) {
	id, ok := budgetID(c)
	if !ok {
		return
	}
	f, ok := budgetFilter(c)
	if !ok {
		return
	}
	var in services.BudgetCellInput
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(400, gin.H{"error": "Некорректная ячейка"})
		return
	}
	if err := services.SaveBudgetCell(id, in, c.GetString("username"), f, c.Request.Method == "DELETE"); err != nil {
		budgetError(c, err)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func GetBudgetPromos(c *gin.Context) {
	f, ok := budgetFilter(c)
	if !ok {
		return
	}
	q, _ := strconv.Atoi(c.Query("quarter"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	r, err := services.BudgetPromosView(f, c.DefaultQuery("version", "LIVE"), c.Query("compare"), c.Query("brand"), q, c.Query("status"), c.Query("type"), page)
	if err != nil {
		budgetError(c, err)
		return
	}
	c.JSON(200, r)
}
func SaveBudgetPromos(c *gin.Context) {
	id, ok := budgetID(c)
	if !ok {
		return
	}
	var in struct {
		IDs       []int  `json:"promo_ids"`
		Included  bool   `json:"included"`
		UpdatedAt string `json:"updated_at"`
	}
	if c.ShouldBindJSON(&in) != nil || len(in.IDs) == 0 || len(in.IDs) > 1000 {
		c.JSON(422, gin.H{"error": "Выберите от 1 до 1000 промо"})
		return
	}
	if err := services.SaveBudgetPromos(id, in.IDs, in.Included, in.UpdatedAt, c.GetString("username")); err != nil {
		budgetError(c, err)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func ExportBudget(c *gin.Context) {
	f, ok := budgetFilter(c)
	if !ok {
		return
	}
	code := c.DefaultQuery("version", "LIVE")
	compare := c.Query("compare")
	book, err := services.BuildBudgetExcel(f, code, compare, c.Query("delta"), c.GetString("username"))
	if err != nil {
		budgetError(c, err)
		return
	}
	defer book.Close()
	buf, err := book.WriteToBuffer()
	if err != nil {
		budgetError(c, err)
		return
	}
	name := fmt.Sprintf("budget-%d-%s", f.Year, code)
	if compare != "" {
		name += "_vs_" + compare
	}
	name += "-" + time.Now().Format("2006-01-02") + ".xlsx"
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	c.Data(200, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}

func SaveBudgetSources(c *gin.Context) {
	id, ok := budgetID(c)
	if !ok {
		return
	}
	var in struct {
		Sources   models.BudgetSources `json:"sources"`
		UpdatedAt string               `json:"updated_at"`
	}
	if c.ShouldBindJSON(&in) != nil || !services.ValidBudgetSources(in.Sources) {
		c.JSON(422, gin.H{"error": "Выберите источники для четырёх кварталов"})
		return
	}
	if err := repository.WriteBudget(repository.BudgetWrite{ID: id, Expected: in.UpdatedAt, Who: c.GetString("username"), Action: "sources", Sources: &in.Sources}); err != nil {
		budgetError(c, err)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
