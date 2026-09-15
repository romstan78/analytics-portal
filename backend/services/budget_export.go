package services

import (
	"fmt"
	"strings"
	"time"

	"backend/models"

	"github.com/xuri/excelize/v2"
)

func BuildBudgetExcel(f BudgetFilter, code, compare, delta, who string) (*excelize.File, error) {
	r, p, err := budgetLoad(f, code)
	if err != nil {
		return nil, err
	}
	var other *models.BudgetResponse
	var otherPromos []models.BudgetPromo
	if compare != "" {
		other, otherPromos, err = budgetLoad(f, compare)
		if err != nil {
			return nil, err
		}
		budgetCompare(r, other, delta)
	}
	return renderBudgetExcel(f, code, compare, delta, who, r, p, other, otherPromos)
}
func renderBudgetExcel(f BudgetFilter, code, compare, delta, who string, r *models.BudgetResponse, p []models.BudgetPromo, other *models.BudgetResponse, otherPromos []models.BudgetPromo) (*excelize.File, error) {
	var err error
	book := excelize.NewFile()
	book.SetSheetName("Sheet1", "Бюджет")
	for _, name := range []string{"Данные", "Промо", "Параметры"} {
		if _, err = book.NewSheet(name); err != nil {
			book.Close()
			return nil, err
		}
	}
	failed := error(nil)
	check := func(e error) {
		if e != nil && failed == nil {
			failed = e
		}
	}
	put := func(sheet string, row, col int, v interface{}) {
		cell, _ := excelize.CoordinatesToCellName(col, row)
		check(book.SetCellValue(sheet, cell, v))
	}
	moneyFormat := `#,##0.0,,;-#,##0.0,,;0.0`
	pctFormat := `0.0" %"`
	ppFormat := `0.0" п.п."`

	head, _ := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}, Fill: excelize.Fill{Type: "pattern", Color: []string{"E2E8F0"}, Pattern: 1}})
	put("Бюджет", 1, 1, "Бренд / показатель · млн ₽")
	width := 1
	if compare != "" {
		width = 3
	}
	for q, name := range []string{"Q1", "Q2", "Q3", "Q4", "Год"} {
		col := 2 + q*width
		put("Бюджет", 1, col, name)
		if width == 3 {
			a, _ := excelize.CoordinatesToCellName(col, 1)
			b, _ := excelize.CoordinatesToCellName(col+2, 1)
			check(book.MergeCell("Бюджет", a, b))
			put("Бюджет", 2, col, code)
			put("Бюджет", 2, col+1, compare)
			put("Бюджет", 2, col+2, "Δ")
		}
	}
	endCol, _ := excelize.ColumnNumberToName(1 + 5*width)
	check(book.SetCellStyle("Бюджет", "A1", endCol+"2", head))
	check(book.SetColWidth("Бюджет", "A", "A", 44))
	check(book.SetColWidth("Бюджет", "B", endCol, 16))
	check(book.SetPanes("Бюджет", &excelize.Panes{Freeze: true, XSplit: 1, YSplit: 2, TopLeftCell: "B3", ActivePane: "bottomRight"}))
	styles := map[string]int{}
	row := 3
	writeBrand := func(b *models.BudgetBrand, level uint8, hidden bool) {
		items := []struct {
			name string
			line *models.BudgetLine
			kind string
		}{{"ТО", &b.To, "to"}, {"Инвестиции", &b.Investments, "inv"}, {"GTN · контракт", &b.GtnContract, "inv"}, {"OPEX · контракт", &b.OpexContract, "inv"}, {"GTN · промо", &b.GtnPromo, "inv"}, {"OPEX · промо", &b.OpexPromo, "inv"}}
		if f.Base != "" && f.Base != "reg-contract" && f.Base != "reg-olap" {
			items = append(items, struct {
				name string
				line *models.BudgetLine
				kind string
			}{"Продажи", &b.Sales, "sales"})
		}
		items = append(items, struct {
			name string
			line *models.BudgetLine
			kind string
		}{"%", &b.Pct, "pct"})
		for i, item := range items {
			if (strings.Contains(item.name, "контракт") && f.Source == "promo") ||
				(strings.Contains(item.name, "промо") && f.Source == "contract") ||
				(strings.HasPrefix(item.name, "GTN") && f.Kind == "opex") ||
				(strings.HasPrefix(item.name, "OPEX") && f.Kind == "gtn") {
				continue
			}
			label := item.name
			if i == 0 {
				label = b.Brand + " · ТО"
			}
			put("Бюджет", row, 1, label)
			outline := level
			if level == 0 && i > 1 && item.kind != "pct" {
				outline = 1
			}
			if hidden || (!f.Composition && (item.name == "GTN · контракт" || item.name == "OPEX · контракт" || item.name == "GTN · промо" || item.name == "OPEX · промо")) {
				check(book.SetRowVisible("Бюджет", row, false))
			}
			if outline > 0 {
				check(book.SetRowOutlineLevel("Бюджет", row, outline))
			}
			for q, c := range append(append([]models.BudgetCell{}, item.line.Q...), item.line.Year) {
				col := 2 + q*width
				values := []*float64{c.A}
				if width == 3 {
					values = append(values, c.B, c.Delta)
				}
				for k, v := range values {
					cell, _ := excelize.CoordinatesToCellName(col+k, row)
					if v != nil {
						put("Бюджет", row, col+k, *v)
					}
					format := moneyFormat
					if item.kind == "pct" || k == 2 && delta == "pct" {
						format = pctFormat
					}
					if item.kind == "pct" && k == 2 {
						format = ppFormat
					}
					state := ""
					if q < 4 {
						state = r.QuarterStates[q]
						if k == 1 && other != nil {
							state = other.QuarterStates[q]
						}
					}
					fill := "FFFFFF"
					if state == "current" {
						fill = "F5F3FF"
					}
					if state == "future" {
						fill = "EEF2FF"
					}
					if c.Edited != nil && k == 0 {
						fill = "FFFBEB"
					}
					color := "334155"
					if k == 2 && v != nil && *v != 0 && (item.kind == "to" || item.kind == "pct") {
						good := *v > 0
						if item.kind == "pct" {
							good = *v < 0
						}
						if good {
							color = "15803D"
						} else {
							color = "B91C1C"
						}
					}
					styleKey := format + fill + color
					style, ok := styles[styleKey]
					if !ok {
						var e error
						style, e = book.NewStyle(&excelize.Style{CustomNumFmt: &format, Font: &excelize.Font{Color: color}, Fill: excelize.Fill{Type: "pattern", Color: []string{fill}, Pattern: 1}})
						check(e)
						styles[styleKey] = style
					}
					check(book.SetCellStyle("Бюджет", cell, cell, style))
				}
				if c.Edited != nil {
					cell, _ := excelize.CoordinatesToCellName(col, row)
					check(book.AddComment("Бюджет", excelize.Comment{Cell: cell, Author: c.Edited.Who, Text: fmt.Sprintf("Реестр %.2f → %.2f; %s", c.Edited.Base, c.Edited.Value, c.Edited.When)}))
				}
			}
			row++
		}
	}
	for i := range r.Brands {
		if r.Brands[i].Brand == "Нераспределённый остаток пула" {
			continue
		}
		writeBrand(&r.Brands[i], 0, false)
		expanded := false
		for _, brand := range f.ExpandedBrands {
			if brand == r.Brands[i].Brand {
				expanded = true
			}
		}
		for j := range r.Brands[i].Networks {
			writeBrand(&r.Brands[i].Networks[j], 2, !expanded)
		}
	}
	writeBrand(&r.Total, 0, false)
	headers := []string{"Версия", "Сеть ID", "Сеть", "Бренд", "Квартал", "Показатель", "Значение, руб / %"}
	for i, h := range headers {
		put("Данные", 1, i+1, h)
	}
	dr := 2
	for _, v := range []*models.BudgetResponse{r, other} {
		if v == nil {
			continue
		}
		for bi := range v.Brands {
			b := &v.Brands[bi]
			for ni := range b.Networks {
				n := &b.Networks[ni]
				lines := budgetLines(n)
				lines["inv"] = &n.Investments
				lines["pct"] = &n.Pct
				lines["sales"] = &n.Sales
				for metric, line := range lines {
					for q, c := range line.Q {
						values := []interface{}{v.Version, n.NetworkID, n.Brand, b.Brand, q + 1, metric, nil}
						if c.A != nil {
							values[6] = *c.A
						}
						for col, value := range values {
							put("Данные", dr, col+1, value)
						}
						dr++
					}
				}
			}
		}
	}
	ph := []string{"Версия", "Промо ID", "Сеть", "Бренд", "Квартал", "Месяц", "Тип", "Статус", "Механика", "План, руб", "Факт, руб", "Бюджет, руб", "Бюджет без НДС, руб", "В бюджете"}
	for i, h := range ph {
		put("Промо", 1, i+1, h)
	}
	pr := 2
	for index, rows := range [][]models.BudgetPromo{p, otherPromos} {
		ver := code
		if index == 1 {
			ver = compare
		}
		for _, v := range rows {
			values := []interface{}{ver, v.ID, v.Network, v.Brand, v.Quarter, v.Month, v.Type, v.Status, v.Mechanics, v.PlanRub, v.FactRub, v.BudgetRub, v.BudgetNet, v.Included}
			for col, value := range values {
				put("Промо", pr, col+1, value)
			}
			pr++
		}
	}
	params := [][2]string{{"Год", fmt.Sprint(f.Year)}, {"Версия", code}, {"Сравнение", compare}, {"Δ", delta}, {"База", f.Base}, {"Источник", f.Source}, {"Тип инвестиций", f.Kind}, {"Типы сетей", strings.Join(f.NetworkTypes, ", ")}, {"С НДС", fmt.Sprint(f.Gross)}, {"Сформировал", who}, {"Сформировано", time.Now().Format(time.RFC3339)}, {"Заморожена", r.VersionInfo.FrozenAt}, {"ТО Q1–Q4", strings.Join(r.VersionInfo.Sources.To, ", ")}, {"GTN Q1–Q4", strings.Join(r.VersionInfo.Sources.Investments, ", ")}}
	for _, b := range r.Brands {
		for metric, line := range budgetLines(&b) {
			for q, c := range line.Q {
				if c.Edited != nil {
					params = append(params, [2]string{"Правка", fmt.Sprintf("%s Q%d %s: %.2f → %.2f; %s %s", b.Brand, q+1, metric, c.Edited.Base, c.Edited.Value, c.Edited.Who, c.Edited.When)})
				}
			}
		}
	}
	for _, v := range p {
		if !v.Included {
			params = append(params, [2]string{"Исключено промо", fmt.Sprint(v.ID)})
		}
	}
	for i, pair := range params {
		put("Параметры", i+1, 1, pair[0])
		put("Параметры", i+1, 2, pair[1])
	}
	check(book.SetColWidth("Параметры", "A", "A", 28))
	check(book.SetColWidth("Параметры", "B", "B", 100))
	for _, sheet := range []string{"Данные", "Промо"} {
		check(book.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2"}))
		check(book.SetColWidth(sheet, "A", "N", 23))
		check(book.SetCellStyle(sheet, "A1", "N1", head))
	}
	if failed != nil {
		book.Close()
		return nil, failed
	}
	return book, nil
}
