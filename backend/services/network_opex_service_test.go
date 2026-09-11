package services

import (
	"math"
	"testing"

	"backend/models"
)

// Раскладка квартала на месяцы — единственная формула бюджета OPEX, и у неё два
// обещания: ни одно значение не мельче копейки, а сумма трёх месяцев равна
// введённому кварталу. Второе важнее: если оно нарушится, КАМ увидит в базе не
// ту цифру, которую согласовал.
func TestSplitOpexKopecks(t *testing.T) {
	cases := []struct {
		name   string
		amount float64
		want   [3]float64
	}{
		{"делится ровно", 300, [3]float64{100, 100, 100}},
		{"остаток копейки в последний месяц", 100.01, [3]float64{33.33, 33.33, 33.35}},
		{"остаток две копейки", 100, [3]float64{33.33, 33.33, 33.34}},
		{"отрицательный бюджет", -100.01, [3]float64{-33.33, -33.33, -33.35}},
		{"ноль", 0, [3]float64{0, 0, 0}},
		{"копейка целиком уходит в последний месяц", 0.01, [3]float64{0, 0, 0.01}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := splitOpexKopecks(testCase.amount)
			if got != testCase.want {
				t.Fatalf("раскладка %v = %v, ожидалось %v", testCase.amount, got, testCase.want)
			}
			sum := round2(got[0] + got[1] + got[2])
			if sum != round2(testCase.amount) {
				t.Fatalf("сумма месяцев = %v, введено %v", sum, testCase.amount)
			}
		})
	}
}

// Случайных сумм здесь нет намеренно: проверяются границы, на которых правило
// и ломается — нечётные копейки в обе стороны знака.
func TestSplitOpexKopecksKeepsSum(t *testing.T) {
	for kopecks := -250; kopecks <= 250; kopecks++ {
		amount := float64(kopecks) / 100
		got := splitOpexKopecks(amount)
		if sum := round2(got[0] + got[1] + got[2]); sum != round2(amount) {
			t.Fatalf("сумма месяцев %v не равна введённому %v", sum, amount)
		}
		for _, value := range got {
			if math.Abs(value*100-math.Round(value*100)) > 1e-9 {
				t.Fatalf("месяц %v мельче копейки при вводе %v", value, amount)
			}
		}
	}
}

// Обе базы НДС раскладываются своим делением, поэтому сумма месяцев сходится
// с кварталом и в «без НДС» — потребителю этой колонки ничего не досчитывать.
func TestNetworkOpexMonthlyRowsVAT(t *testing.T) {
	months := NetworkOpexMonthlyRows(2, 120, true, 20)
	if len(months) != 3 {
		t.Fatalf("месяцев в квартале = %d", len(months))
	}
	if months[0].Month != 4 || months[2].Month != 6 {
		t.Fatalf("месяцы Q2 = %d…%d", months[0].Month, months[2].Month)
	}

	gross := round2(months[0].AmountRub + months[1].AmountRub + months[2].AmountRub)
	if gross != 120 {
		t.Fatalf("сумма с НДС = %v, введено 120", gross)
	}
	net := round2(months[0].AmountNet + months[1].AmountNet + months[2].AmountNet)
	if net != NetRub(120, true, 20) {
		t.Fatalf("сумма без НДС = %v, ожидалось %v", net, NetRub(120, true, 20))
	}
}

// Сеть без НДС в квартале получает равные базы — то же обещание, что и у
// инвестиций GTN: колонка «без НДС» всегда пригодна для сложения сетей.
func TestNetworkOpexMonthlyRowsWithoutVAT(t *testing.T) {
	months := NetworkOpexMonthlyRows(1, 99.99, false, 0)
	for _, month := range months {
		if month.AmountRub != month.AmountNet {
			t.Fatalf("месяц %d: с НДС %v, без НДС %v", month.Month, month.AmountRub, month.AmountNet)
		}
	}
}

func opexRow(month int, brand, article string, amount, net float64) models.NetworkOpexBudgetRow {
	return models.NetworkOpexBudgetRow{
		Year: 2026, Month: month, BrandAS: brand, Article: article,
		AmountRub: amount, AmountNet: net, UpdatedAt: "2026-09-10 12:00:00.000",
	}
}

// Квартальная ячейка собирается из месяцев, и введённое значение возвращается
// без изменений — иначе вкладка показывала бы не то, что сохранили.
func TestBuildNetworkOpexResponse(t *testing.T) {
	network := models.Network{ID: 7, Name: "Сеть", VATIncluded: true, VATRate: 20}
	months := NetworkOpexMonthlyRows(1, 100.01, true, 20)
	rows := make([]models.NetworkOpexBudgetRow, 0, 3)
	for _, month := range months {
		rows = append(rows, opexRow(month.Month, "Альфа", "display", month.AmountRub, month.AmountNet))
	}
	// Бренд с бюджетом, но уже без плана: его деньги обязаны остаться на экране.
	rows = append(rows, opexRow(5, "Гамма", "reports", 50, 41.67))

	response := BuildNetworkOpexResponse(network, 2026, []string{"Альфа", "Бета"}, rows)

	if len(response.Articles) != 5 {
		t.Fatalf("статей в ответе = %d", len(response.Articles))
	}
	wantBrands := []string{"Альфа", "Бета", "Гамма"}
	if len(response.Brands) != len(wantBrands) {
		t.Fatalf("бренды сетки = %v, ожидалось %v", response.Brands, wantBrands)
	}
	for index, brand := range wantBrands {
		if response.Brands[index] != brand {
			t.Fatalf("бренды сетки = %v, ожидалось %v", response.Brands, wantBrands)
		}
	}

	var display *models.NetworkOpexCell
	for index := range response.Cells {
		if response.Cells[index].BrandAS == "Альфа" && response.Cells[index].Article == "display" {
			display = &response.Cells[index]
		}
	}
	if display == nil {
		t.Fatal("ячейка «Альфа · выкладка» не собрана")
	}
	if display.AmountRub != 100.01 {
		t.Fatalf("квартальная сумма = %v, введено 100.01", display.AmountRub)
	}
	if display.Quarter != 1 || len(display.Months) != 3 {
		t.Fatalf("ячейка Q%d с %d месяцами", display.Quarter, len(display.Months))
	}
	if display.Months[0].Month != 1 || display.Months[2].Month != 3 {
		t.Fatalf("месяцы ячейки = %d…%d", display.Months[0].Month, display.Months[2].Month)
	}
	if display.UpdatedAt == "" {
		t.Fatal("версия ячейки пуста — оптимистичная блокировка не сработает")
	}

	if response.Totals.AmountRub != round2(100.01+50) {
		t.Fatalf("итог года = %v", response.Totals.AmountRub)
	}
	if response.Totals.Quarters[0].AmountRub != 100.01 {
		t.Fatalf("итог Q1 = %v", response.Totals.Quarters[0].AmountRub)
	}
	if response.Totals.Quarters[1].AmountRub != 50 {
		t.Fatalf("итог Q2 = %v", response.Totals.Quarters[1].AmountRub)
	}

	for _, brand := range response.ByBrand {
		switch brand.BrandAS {
		case "Альфа":
			if brand.AmountRub != 100.01 || brand.Quarters[0].AmountRub != 100.01 {
				t.Fatalf("итог «Альфа» = %v", brand.AmountRub)
			}
		case "Бета":
			if brand.AmountRub != 0 {
				t.Fatalf("бренд без бюджета принёс %v", brand.AmountRub)
			}
		case "Гамма":
			if brand.Quarters[1].AmountRub != 50 {
				t.Fatalf("итог «Гамма» Q2 = %v", brand.Quarters[1].AmountRub)
			}
		}
	}
}
