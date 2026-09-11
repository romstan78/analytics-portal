package services

import (
	"math"
	"sort"
	"strings"

	"backend/models"
)

// ─── Бюджет OPEX по контракту ───────────────────────────────────────────────
//
// Второй механизм инвестиций реестра, рядом с процентом от товарооборота.
// Общего расчёта у них нет: процент зависит от объёма и проверяется порогом
// выполнения, бюджет OPEX — это согласованная сумма за услуги сети, и она не
// зависит ни от объёма, ни от выполнения плана.
//
// Ввод квартальный, хранение помесячное, и раскладка между ними — единственная
// формула этого файла.

// opexArticles — статьи бюджета в порядке показа.
//
// Копия этого списка стоит в CK_NetworkOpexBudgets_article (миграция 031):
// значения, которого нет в обоих местах, база не примет. Дублирование
// осознанное — БД обязана защищать себя сама, даже когда пишет не этот код.
var opexArticles = []models.NetworkOpexArticle{
	{Code: "ntz_bdn", Label: "Поддержание НТЗ/БДН"},
	{Code: "display", Label: "Выкладка"},
	{Code: "reports", Label: "Предоставление отчётов"},
	{Code: "fixed_promo", Label: "Акция с фиксированной стоимостью"},
	{Code: "product_card", Label: "Размещение карточки товара на сайте сети"},
}

// NetworkOpexArticles — статьи бюджета для интерфейса, в порядке показа.
func NetworkOpexArticles() []models.NetworkOpexArticle {
	articles := make([]models.NetworkOpexArticle, len(opexArticles))
	copy(articles, opexArticles)
	return articles
}

// opexArticleOrder — место статьи в порядке показа; −1 для неизвестной.
func opexArticleOrder(code string) int {
	for index, article := range opexArticles {
		if article.Code == code {
			return index
		}
	}
	return -1
}

// IsNetworkOpexArticle — код статьи известен.
func IsNetworkOpexArticle(code string) bool {
	return opexArticleOrder(code) >= 0
}

// splitOpexKopecks делит квартальную сумму на три месяца как можно ровнее.
//
// Счёт идёт в копейках: мельче копейки месяц не получает ничего, а остаток от
// деления — не больше двух копеек — уходит в последний месяц квартала. Поэтому
// сумма трёх месяцев равна исходной по построению, а не с точностью до
// округления, и введённое КАМом число возвращается из базы без изменений.
//
// Отрицательный бюджет раскладывается тем же правилом: целочисленное деление в
// Go усекает к нулю, так что остаток сохраняет знак суммы и последний месяц
// по-прежнему крупнее по модулю, а не меньше.
func splitOpexKopecks(amountRub float64) [3]float64 {
	total := int64(math.Round(amountRub * 100))
	share := total / 3
	return [3]float64{
		float64(share) / 100,
		float64(share) / 100,
		float64(total-2*share) / 100,
	}
}

// NetworkOpexMonthlyRows раскладывает квартальную сумму на месяцы квартала.
//
// Базы НДС раскладываются каждая своим делением, а не пересчётом месяца из
// месяца: так сумма месяцев равна квартальной в обеих базах сразу, и потребителю
// колонки «без НДС» нечего досчитывать.
func NetworkOpexMonthlyRows(
	quarter int,
	amountRub float64,
	vatIncluded bool,
	vatRate float64,
) []models.NetworkOpexMonth {
	gross := splitOpexKopecks(round2(amountRub))
	net := splitOpexKopecks(NetRub(amountRub, vatIncluded, vatRate))
	months := make([]models.NetworkOpexMonth, 0, 3)
	for index := 0; index < 3; index++ {
		months = append(months, models.NetworkOpexMonth{
			Month:     (quarter-1)*3 + 1 + index,
			AmountRub: gross[index],
			AmountNet: net[index],
		})
	}
	return months
}

// opexCellKey — ячейка ввода: квартал, бренд, статья.
type opexCellKey struct {
	quarter int
	brand   string
	article string
}

// BuildNetworkOpexResponse собирает вкладку из хранимых месячных строк.
//
// Квартальная сумма — сумма месяцев, а не отдельно сохранённое число: второй
// копии введённого значения в базе нет, и расходиться ей не с чем.
//
// Бренды сетки — бренды плана года плюс те, у кого бюджет уже заведён. Второе
// слагаемое нужно ради денег: бренд, выведенный из плана, иначе унёс бы свой
// бюджет с экрана, оставив его в базе и в витрине.
// Настройки НДС в аргументы не нужны: обе базы уже лежат в хранимых строках —
// их посчитали при записи, по ставке того квартала, в который бюджет заведён.
func BuildNetworkOpexResponse(
	network models.Network,
	year int,
	planBrands []string,
	rows []models.NetworkOpexBudgetRow,
) models.NetworkOpexResponse {
	cells := map[opexCellKey]*models.NetworkOpexCell{}
	brands := map[string]bool{}
	for _, brand := range planBrands {
		if trimmed := strings.TrimSpace(brand); trimmed != "" {
			brands[trimmed] = true
		}
	}

	for _, row := range rows {
		brand := strings.TrimSpace(row.BrandAS)
		if brand == "" || !IsNetworkOpexArticle(row.Article) {
			continue
		}
		brands[brand] = true
		key := opexCellKey{quarter: (row.Month-1)/3 + 1, brand: brand, article: row.Article}
		cell := cells[key]
		if cell == nil {
			cell = &models.NetworkOpexCell{
				Quarter: key.quarter, BrandAS: brand, Article: row.Article,
				Months: []models.NetworkOpexMonth{},
			}
			cells[key] = cell
		}
		cell.AmountRub = round2(cell.AmountRub + row.AmountRub)
		cell.AmountNet = round2(cell.AmountNet + row.AmountNet)
		cell.Months = append(cell.Months, models.NetworkOpexMonth{
			Month: row.Month, AmountRub: row.AmountRub, AmountNet: row.AmountNet,
		})
		// Версия ячейки — самая свежая из её месяцев: правка любого из них
		// обязана обесценить ту копию, что лежит в чужой вкладке.
		if row.UpdatedAt > cell.UpdatedAt {
			cell.UpdatedAt = row.UpdatedAt
		}
	}

	list := make([]models.NetworkOpexCell, 0, len(cells))
	for _, cell := range cells {
		sort.Slice(cell.Months, func(i, j int) bool { return cell.Months[i].Month < cell.Months[j].Month })
		list = append(list, *cell)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].BrandAS != list[j].BrandAS {
			return list[i].BrandAS < list[j].BrandAS
		}
		if list[i].Quarter != list[j].Quarter {
			return list[i].Quarter < list[j].Quarter
		}
		return opexArticleOrder(list[i].Article) < opexArticleOrder(list[j].Article)
	})

	brandNames := make([]string, 0, len(brands))
	for brand := range brands {
		brandNames = append(brandNames, brand)
	}
	sort.Strings(brandNames)

	// Итоги: по бренду и по всей сетке. Считаются здесь, а не в интерфейсе, —
	// база НДС у итога та же, что у ячеек, и складывать их надо одним правилом.
	byBrand := make([]models.NetworkOpexBrandTotals, 0, len(brandNames))
	totals := models.NetworkOpexTotals{Quarters: emptyOpexQuarters()}
	for _, brand := range brandNames {
		brandTotals := models.NetworkOpexBrandTotals{BrandAS: brand, Quarters: emptyOpexQuarters()}
		for _, cell := range list {
			if cell.BrandAS != brand {
				continue
			}
			index := cell.Quarter - 1
			brandTotals.Quarters[index].AmountRub = round2(brandTotals.Quarters[index].AmountRub + cell.AmountRub)
			brandTotals.Quarters[index].AmountNet = round2(brandTotals.Quarters[index].AmountNet + cell.AmountNet)
			brandTotals.AmountRub = round2(brandTotals.AmountRub + cell.AmountRub)
			brandTotals.AmountNet = round2(brandTotals.AmountNet + cell.AmountNet)
			totals.Quarters[index].AmountRub = round2(totals.Quarters[index].AmountRub + cell.AmountRub)
			totals.Quarters[index].AmountNet = round2(totals.Quarters[index].AmountNet + cell.AmountNet)
			totals.AmountRub = round2(totals.AmountRub + cell.AmountRub)
			totals.AmountNet = round2(totals.AmountNet + cell.AmountNet)
		}
		byBrand = append(byBrand, brandTotals)
	}

	return models.NetworkOpexResponse{
		Network:  network,
		Year:     year,
		Articles: NetworkOpexArticles(),
		Brands:   brandNames,
		Cells:    list,
		ByBrand:  byBrand,
		Totals:   totals,
	}
}

func emptyOpexQuarters() []models.NetworkOpexQuarterTotals {
	quarters := make([]models.NetworkOpexQuarterTotals, 0, 4)
	for quarter := 1; quarter <= 4; quarter++ {
		quarters = append(quarters, models.NetworkOpexQuarterTotals{Quarter: quarter})
	}
	return quarters
}
