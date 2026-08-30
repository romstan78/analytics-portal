package services

import (
	"math"
	"testing"
)

// Формулы карточки промо. Раньше те же проверки жили в браузере
// (frontend/src/utils/calcUtils.test.ts) рядом со второй реализацией формул;
// после переноса расчёта на сервер копия удалена, а случаи перенесены сюда —
// иначе вместе с дубликатом пропало бы и покрытие.

func assertClose(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.001 {
		t.Fatalf("%s = %v, ожидалось %v", name, got, want)
	}
}

// calcWith считает поля по входу, минуя обращения к БД: контекст задаётся явно.
func calcWith(input PromoInputDTO, gm float64) CalculatedFields {
	return CalculateFields(&input, CalculationContext{GM: gm})
}

func TestCalculateFieldsPlanBasics(t *testing.T) {
	got := calcWith(PromoInputDTO{
		PlanPromoUnits: 100, ContractPrice: 200, Year: 2026, Month: 1,
	}, 1)
	assertClose(t, "plan_promo_rub", got.PlanPromoRub, 20000)

	got = calcWith(PromoInputDTO{
		PlanPromoUnits: 150, ContractPrice: 100, BaselineUnits: 100, Year: 2026, Month: 1,
	}, 1)
	assertClose(t, "plan_promo_uplift_units", got.PlanPromoUpliftUnits, 50)
	assertClose(t, "plan_promo_uplift_rub", got.PlanPromoUpliftRub, 5000)

	got = calcWith(PromoInputDTO{
		ContractPrice: 300, BaselineUnits: 100, Year: 2026, Month: 1,
	}, 1)
	assertClose(t, "baseline_rub", got.BaselineRub, 30000)
}

// ROI = (uplift_rub / investments) * gm * 100 - 100.
func TestCalculateFieldsPlanROI(t *testing.T) {
	tests := []struct {
		name                                string
		units, price, baseline, investments float64
		gm                                  float64
		want                                float64
	}{
		{name: "инвестиции не окупились", units: 150, price: 200, baseline: 100, investments: 50000, gm: 0.5, want: -90},
		{name: "ровно окупились", units: 300, price: 200, baseline: 100, investments: 20000, gm: 0.5, want: 0},
		{name: "GM по умолчанию", units: 200, price: 150, baseline: 100, investments: 30000, gm: 1, want: -50},
		// Деление на ноль: без инвестиций ROI не определён и остаётся нулём.
		{name: "нулевые инвестиции", units: 200, price: 100, baseline: 100, investments: 0, gm: 0.56, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcWith(PromoInputDTO{
				PlanPromoUnits: tt.units, ContractPrice: tt.price, BaselineUnits: tt.baseline,
				PlanInvestmentsRub: tt.investments, Year: 2026, Month: 1,
			}, tt.gm)
			assertClose(t, "plan_roi", got.PlanROI, tt.want)
		})
	}
}

// План ниже базовой линии — промо в минус; отрицательный uplift не обнуляется.
func TestCalculateFieldsNegativeUplift(t *testing.T) {
	got := calcWith(PromoInputDTO{
		PlanPromoUnits: 80, ContractPrice: 200, BaselineUnits: 100,
		PlanInvestmentsRub: 5000, Year: 2026, Month: 1,
	}, 1)
	assertClose(t, "plan_promo_uplift_units", got.PlanPromoUpliftUnits, -20)
	assertClose(t, "plan_promo_uplift_rub", got.PlanPromoUpliftRub, -4000)
}

func TestCalculateFieldsActualROI(t *testing.T) {
	got := calcWith(PromoInputDTO{
		ActualPromoSalesUnits: 150, ContractPrice: 200, BaselineUnits: 100,
		ActualInvestments: 50000, Year: 2026, Month: 1,
	}, 0.5)
	assertClose(t, "actual_roi", got.ActualROI, -90)

	// Без фактических инвестиций ROI остаётся нулём, а не уходит в бесконечность.
	got = calcWith(PromoInputDTO{
		ActualPromoSalesUnits: 150, ContractPrice: 200, BaselineUnits: 100,
		ActualInvestments: 0, Year: 2026, Month: 1,
	}, 0.5)
	assertClose(t, "actual_roi", got.ActualROI, 0)
}

// Факт считается от факта продаж — так же, как план от plan_promo_units.
func TestCalculateFieldsActualFromSales(t *testing.T) {
	got := calcWith(PromoInputDTO{
		ActualPromoSalesUnits: 150, ContractPrice: 200, BaselineUnits: 100, Year: 2026, Month: 1,
	}, 1)
	assertClose(t, "actual_promo_rub", got.ActualPromoRub, 30000)
	assertClose(t, "actual_promo_uplift_units", got.ActualPromoUpliftUnits, 50)
	assertClose(t, "actual_promo_uplift_rub", got.ActualPromoUpliftRub, 10000)
}

// Скорректированный baseline вытесняет плановый: на факте сравнивают с ним.
func TestCalculateFieldsActualUsesCorrectedBaseline(t *testing.T) {
	got := calcWith(PromoInputDTO{
		ActualPromoSalesUnits: 150, ContractPrice: 200, BaselineUnits: 100,
		ActualCorrectedBaseline: 120, Year: 2026, Month: 1,
	}, 1)
	assertClose(t, "actual_promo_uplift_units", got.ActualPromoUpliftUnits, 30)
	assertClose(t, "actual_promo_uplift_rub", got.ActualPromoUpliftRub, 6000)
}

// Без факта продаж фактических показателей нет: uplift не уходит в минус на
// величину baseline, иначе промо выглядело бы убыточным до сбора факта.
func TestCalculateFieldsActualEmptyWithoutSales(t *testing.T) {
	got := calcWith(PromoInputDTO{
		ContractPrice: 200, BaselineUnits: 100, ActualInvestments: 5000, Year: 2026, Month: 1,
	}, 1)
	assertClose(t, "actual_promo_rub", got.ActualPromoRub, 0)
	assertClose(t, "actual_promo_uplift_units", got.ActualPromoUpliftUnits, 0)
	assertClose(t, "actual_promo_uplift_rub", got.ActualPromoUpliftRub, 0)
}

// Строка для БД должна нести расчётный факт, а не то, что прислал клиент:
// именно она уходит в INSERT/UPDATE.
func TestDTOToDBRowStoresCalculatedActuals(t *testing.T) {
	dto := PromoInputDTO{
		ActualPromoSalesUnits: 150, ContractPrice: 200, BaselineUnits: 100,
		ActualPromoRub: 1, ActualPromoUpliftUnits: 2, ActualPromoUpliftRub: 3,
		Year: 2026, Month: 1,
	}
	row := DTOToDBRow(dto, calcWith(dto, 1))
	if row.ActualPromoRub == nil || row.ActualPromoUpliftUnits == nil || row.ActualPromoUpliftRub == nil {
		t.Fatalf("фактические поля не заполнены: %+v", row)
	}
	assertClose(t, "actual_promo_rub", *row.ActualPromoRub, 30000)
	assertClose(t, "actual_promo_uplift_units", *row.ActualPromoUpliftUnits, 50)
	assertClose(t, "actual_promo_uplift_rub", *row.ActualPromoUpliftRub, 10000)
}

// Пустой факт по-прежнему пишется как NULL, а не как ноль.
func TestDTOToDBRowKeepsEmptyActualsNull(t *testing.T) {
	dto := PromoInputDTO{ContractPrice: 200, BaselineUnits: 100, Year: 2026, Month: 1}
	row := DTOToDBRow(dto, calcWith(dto, 1))
	for name, value := range map[string]*float64{
		"actual_promo_rub":          row.ActualPromoRub,
		"actual_promo_uplift_units": row.ActualPromoUpliftUnits,
		"actual_promo_uplift_rub":   row.ActualPromoUpliftRub,
		"actual_roi":                row.ActualROI,
	} {
		if value != nil {
			t.Fatalf("%s = %v, ожидался NULL", name, *value)
		}
	}
}

func TestCalculateFieldsAllZeroes(t *testing.T) {
	got := calcWith(PromoInputDTO{Year: 2026, Month: 1}, 1)
	for name, value := range map[string]float64{
		"plan_promo_rub":          got.PlanPromoRub,
		"plan_promo_uplift_units": got.PlanPromoUpliftUnits,
		"plan_promo_uplift_rub":   got.PlanPromoUpliftRub,
		"plan_roi":                got.PlanROI,
		"baseline_rub":            got.BaselineRub,
		"actual_roi":              got.ActualROI,
	} {
		assertClose(t, name, value, 0)
	}
}
