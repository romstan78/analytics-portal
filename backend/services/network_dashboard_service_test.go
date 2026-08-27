package services

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"backend/models"
	"backend/repository"
)

// Витрина открывается «сегодня» фиксированной датой: закрытость месяца решает
// не календарь машины, а эта точка. Май 2026 — месяцы 1-4 закрыты, 5 и 6 нет.
var dashboardNow = time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)

func dashboardNetwork(id int, name, kam string) models.Network {
	return models.Network{
		ID: id, Name: name, KAM: models.PtrString(kam), NetworkType: "regular", IsActive: true,
		VATIncluded: true, VATRate: 20,
		Month1Pct: 30, Month2Pct: 30, Month3Pct: 40,
	}
}

// dashboardFilter собирает срез из перечисленных кварталов. Набор, а не
// диапазон: витрина умеет сравнивать и несмежные кварталы.
func dashboardFilter(quarters ...int) repository.NetworkDashboardFilter {
	return repository.NetworkDashboardFilter{Year: 2026, Quarters: quarters}
}

// dashboardCase — вход агрегатора. Прошлый год и промо необязательны: без них
// витрина обязана работать так же, просто без сравнения и без меток.
type dashboardCase struct {
	networks  []models.Network
	plans     []models.NetworkPlan
	facts     []models.NetworkMonthlyFact
	forecasts []models.NetworkForecastLine
	prevPlans []models.NetworkPlan
	prevFacts []models.NetworkMonthlyFact
	promos    []repository.NetworkDashboardPromoRow
}

func (c dashboardCase) data() repository.NetworkDashboardData {
	return repository.NetworkDashboardData{
		Networks: c.networks,
		Current: repository.NetworkDashboardPeriodData{
			Year: 2026, Plans: c.plans, Facts: c.facts, Forecasts: c.forecasts,
		},
		Prev: repository.NetworkDashboardPeriodData{
			Year: 2025, Plans: c.prevPlans, Facts: c.prevFacts,
		},
		Promos:         c.promos,
		AvailableYears: []int{2025, 2026},
	}
}

func dashboardFact(networkID, month int, brand string, rub, investments float64) models.NetworkMonthlyFact {
	return models.NetworkMonthlyFact{
		NetworkID: networkID, Year: 2026, Month: month, BrandAS: brand,
		SKU:     models.PtrString("SKU-1"),
		FactRub: models.PtrFloat(rub), FactInvestmentsRub: models.PtrFloat(investments),
	}
}

func skuFact(networkID, month int, brand, sku string, rub, investments float64) models.NetworkMonthlyFact {
	fact := dashboardFact(networkID, month, brand, rub, investments)
	fact.SKU = models.PtrString(sku)
	return fact
}

func dashboardFactUnits(networkID, month int, brand string, rub, units float64) models.NetworkMonthlyFact {
	fact := dashboardFact(networkID, month, brand, rub, 0)
	fact.FactUnits = models.PtrFloat(units)
	return fact
}

// Квартальные колонки tbl_NetworkPlans заполняются только загрузкой отгрузок и
// сохранением прогноза. Витрина обязана считать факт из помесячных строк, иначе
// сеть, которую никто не открывал, показала бы план без факта.
func TestAggregateNetworkDashboardTakesFactFromMonthlyRows(t *testing.T) {
	testCase := dashboardCase{
		networks: []models.Network{dashboardNetwork(1, "Аптека Плюс", "Иванов")},
		plans: []models.NetworkPlan{
			{
				NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"),
				PlanRub: models.PtrFloat(1000000), InvestmentsPct: models.PtrFloat(10),
			},
		},
		facts: []models.NetworkMonthlyFact{
			dashboardFact(1, 1, "Альфа", 300000, 30000),
			dashboardFact(1, 2, "Альфа", 300000, 30000),
			dashboardFact(1, 3, "Альфа", 200000, 20000),
		},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1), dashboardNow)

	if got.Summary.PlanRub != 1000000 {
		t.Errorf("план = %v, ожидалось 1000000", got.Summary.PlanRub)
	}
	if got.Summary.FactRub != 800000 {
		t.Errorf("факт = %v, ожидалось 800000", got.Summary.FactRub)
	}
	// Квартал закрыт целиком: EAC равен факту.
	if got.Summary.EACRub != 800000 {
		t.Errorf("EAC = %v, ожидалось 800000", got.Summary.EACRub)
	}
	if got.Summary.CompletionPct == nil || *got.Summary.CompletionPct != 80 {
		t.Errorf("выполнение = %v, ожидалось 80", got.Summary.CompletionPct)
	}
	if got.Summary.GapRub != -200000 {
		t.Errorf("разрыв = %v, ожидалось -200000", got.Summary.GapRub)
	}
	if got.Summary.ClosedCells != 3 || got.Summary.ClosedCellsWithFact != 3 {
		t.Errorf("покрытие фактом = %d из %d, ожидалось 3 из 3",
			got.Summary.ClosedCellsWithFact, got.Summary.ClosedCells)
	}
}

// Открытый месяц без официального прогноза не достраивается планом: иначе сеть,
// которую никто не вёл, выглядела бы выполняющей план ровно на 100%.
func TestAggregateNetworkDashboardKeepsOpenMonthsWithoutForecastVisible(t *testing.T) {
	testCase := dashboardCase{
		networks: []models.Network{dashboardNetwork(1, "Аптека Плюс", "Иванов")},
		plans: []models.NetworkPlan{
			{
				NetworkID: 1, Year: 2026, Quarter: 2, BrandAS: brandPtr("Альфа"),
				PlanRub: models.PtrFloat(900000), InvestmentsPct: models.PtrFloat(10),
			},
		},
		// Апрель закрыт и отгружен; май и июнь открыты и не спрогнозированы.
		facts: []models.NetworkMonthlyFact{dashboardFact(1, 4, "Альфа", 300000, 30000)},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(2), dashboardNow)

	if got.Summary.EACRub != 300000 {
		t.Errorf("EAC = %v, ожидалось 300000: план не должен достраивать пустые месяцы", got.Summary.EACRub)
	}
	if got.Summary.OpenCellsWithoutForecast != 2 {
		t.Errorf("открытых месяцев без прогноза = %d, ожидалось 2", got.Summary.OpenCellsWithoutForecast)
	}
	if got.Summary.ClosedCells != 1 {
		t.Errorf("закрытых ячеек = %d, ожидалось 1", got.Summary.ClosedCells)
	}
}

// Официальный прогноз открытого месяца попадает в EAC, а закрытый месяц
// остаётся на факте, даже если прогноз на него сохранён.
func TestAggregateNetworkDashboardUsesForecastForOpenMonthsOnly(t *testing.T) {
	testCase := dashboardCase{
		networks: []models.Network{dashboardNetwork(1, "Аптека Плюс", "Иванов")},
		plans: []models.NetworkPlan{
			{
				NetworkID: 1, Year: 2026, Quarter: 2, BrandAS: brandPtr("Альфа"),
				PlanRub: models.PtrFloat(900000), InvestmentsPct: models.PtrFloat(10),
			},
		},
		facts: []models.NetworkMonthlyFact{dashboardFact(1, 4, "Альфа", 300000, 30000)},
		forecasts: []models.NetworkForecastLine{
			// Апрель закрыт: прогноз на него игнорируется в пользу факта.
			{NetworkID: 1, Year: 2026, Month: 4, BrandAS: "Альфа", ForecastRub: models.PtrFloat(999999)},
			{NetworkID: 1, Year: 2026, Month: 5, BrandAS: "Альфа", ForecastRub: models.PtrFloat(250000)},
			{NetworkID: 1, Year: 2026, Month: 6, BrandAS: "Альфа", ForecastRub: models.PtrFloat(260000)},
		},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(2), dashboardNow)

	if got.Summary.EACRub != 810000 {
		t.Errorf("EAC = %v, ожидалось 810000 (факт 300000 + прогноз 250000 и 260000)", got.Summary.EACRub)
	}
	if got.Summary.OpenCellsWithoutForecast != 0 {
		t.Errorf("открытых месяцев без прогноза = %d, ожидалось 0", got.Summary.OpenCellsWithoutForecast)
	}
}

// Плановым объёмом сети считается обязательство по контракту: валовый пул
// входит целиком, даже если бренды разобрали его не полностью.
func TestAggregateNetworkDashboardUsesContractObligation(t *testing.T) {
	testCase := dashboardCase{
		networks: []models.Network{dashboardNetwork(1, "Аптека Плюс", "Иванов")},
		plans: []models.NetworkPlan{
			{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: nil, PlanRub: models.PtrFloat(1000000)},
			{
				NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"), InGross: true,
				PlanRub: models.PtrFloat(600000), InvestmentsPct: models.PtrFloat(10),
			},
			{
				NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Бета"),
				PlanRub: models.PtrFloat(200000), InvestmentsPct: models.PtrFloat(10),
			},
		},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1), dashboardNow)

	// Пул 1 000 000 целиком плюс отдельный бренд 200 000.
	if got.Summary.PlanRub != 1200000 {
		t.Errorf("план = %v, ожидалось 1200000", got.Summary.PlanRub)
	}
	if got.Summary.UndistributedRub == nil || *got.Summary.UndistributedRub != 400000 {
		t.Errorf("нераспределённый остаток = %v, ожидалось 400000", got.Summary.UndistributedRub)
	}

	// В разрезе брендов остатка пула нет: у строки бренда только свой план.
	planByBrand := map[string]float64{}
	for _, brand := range got.Brands {
		planByBrand[brand.Name] = brand.Metrics.PlanRub
	}
	if planByBrand["Альфа"] != 600000 || planByBrand["Бета"] != 200000 {
		t.Errorf("планы брендов = %v, ожидалось Альфа 600000 и Бета 200000", planByBrand)
	}
}

// Упаковки считаются тем же правилом, что и рубли: валовый пул целиком,
// отдельные бренды сверху.
func TestAggregateNetworkDashboardCountsUnitsWithPoolRule(t *testing.T) {
	testCase := dashboardCase{
		networks: []models.Network{dashboardNetwork(1, "Аптека Плюс", "Иванов")},
		plans: []models.NetworkPlan{
			{
				NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: nil,
				PlanRub: models.PtrFloat(1000000), PlanUnits: models.PtrFloat(10000),
			},
			{
				NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"), InGross: true,
				PlanRub: models.PtrFloat(600000), PlanUnits: models.PtrFloat(6000),
			},
			{
				NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Бета"),
				PlanRub: models.PtrFloat(200000), PlanUnits: models.PtrFloat(2500),
			},
		},
		facts: []models.NetworkMonthlyFact{
			dashboardFactUnits(1, 1, "Альфа", 300000, 3000),
			dashboardFactUnits(1, 2, "Бета", 100000, 1200),
		},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1), dashboardNow)

	// Пул 10 000 целиком плюс отдельный бренд 2 500.
	if got.Summary.PlanUnits != 12500 {
		t.Errorf("план в упаковках = %v, ожидалось 12500", got.Summary.PlanUnits)
	}
	if got.Summary.FactUnits != 4200 {
		t.Errorf("факт в упаковках = %v, ожидалось 4200", got.Summary.FactUnits)
	}
	if got.Summary.EACUnits != 4200 {
		t.Errorf("EAC в упаковках = %v, ожидалось 4200: квартал закрыт", got.Summary.EACUnits)
	}
}

// Сети работают с разными ставками НДС, поэтому отклонение по инвестициям
// считается в базе «без НДС»: иначе разница означала бы разный НДС.
func TestAggregateNetworkDashboardInvestmentVarianceUsesNetBase(t *testing.T) {
	withVAT := dashboardNetwork(1, "С НДС", "Иванов")
	withoutVAT := dashboardNetwork(2, "Без НДС", "Петров")
	withoutVAT.VATIncluded = false

	plan := func(networkID int) models.NetworkPlan {
		return models.NetworkPlan{
			NetworkID: networkID, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"),
			PlanRub: models.PtrFloat(1200000), InvestmentsPct: models.PtrFloat(10),
		}
	}
	testCase := dashboardCase{
		networks: []models.Network{withVAT, withoutVAT},
		plans:    []models.NetworkPlan{plan(1), plan(2)},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1), dashboardNow)

	byName := map[string]models.NetworkDashboardMetrics{}
	for _, network := range got.Networks {
		byName[network.Name] = network.Metrics
	}
	// 120 000 валовыми у обеих; с НДС 20% чистая база — 100 000.
	if byName["С НДС"].PlanInvestmentsRub != 120000 || byName["С НДС"].PlanInvestmentsRubNet != 100000 {
		t.Errorf("инвестиции сети с НДС = %v / %v, ожидалось 120000 / 100000",
			byName["С НДС"].PlanInvestmentsRub, byName["С НДС"].PlanInvestmentsRubNet)
	}
	if byName["Без НДС"].PlanInvestmentsRubNet != 120000 {
		t.Errorf("чистые инвестиции сети без НДС = %v, ожидалось 120000",
			byName["Без НДС"].PlanInvestmentsRubNet)
	}
	if got.Summary.PlanInvestmentsRubNet != 220000 {
		t.Errorf("чистые инвестиции портфеля = %v, ожидалось 220000", got.Summary.PlanInvestmentsRubNet)
	}
}

// Прошлый год считается тем же кодом и тем же диапазоном кварталов.
func TestAggregateNetworkDashboardComparesWithPreviousYear(t *testing.T) {
	testCase := dashboardCase{
		networks: []models.Network{dashboardNetwork(1, "Аптека Плюс", "Иванов")},
		plans: []models.NetworkPlan{
			{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(1000000)},
		},
		facts: []models.NetworkMonthlyFact{dashboardFact(1, 1, "Альфа", 600000, 0)},
		prevPlans: []models.NetworkPlan{
			{NetworkID: 1, Year: 2025, Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(800000)},
		},
		prevFacts: []models.NetworkMonthlyFact{{
			NetworkID: 1, Year: 2025, Month: 1, BrandAS: "Альфа", SKU: models.PtrString("SKU-1"),
			FactRub: models.PtrFloat(500000),
		}},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1), dashboardNow)

	if got.Summary.PrevFactRub == nil || *got.Summary.PrevFactRub != 500000 {
		t.Fatalf("факт прошлого года = %v, ожидалось 500000", got.Summary.PrevFactRub)
	}
	if got.Summary.FactYoYPct == nil || *got.Summary.FactYoYPct != 20 {
		t.Errorf("прирост факта = %v, ожидалось 20", got.Summary.FactYoYPct)
	}
	if got.Summary.PrevPlanRub == nil || *got.Summary.PrevPlanRub != 800000 {
		t.Errorf("план прошлого года = %v, ожидалось 800000", got.Summary.PrevPlanRub)
	}
	if got.Summary.PlanYoYPct == nil || *got.Summary.PlanYoYPct != 25 {
		t.Errorf("прирост плана = %v, ожидалось 25", got.Summary.PlanYoYPct)
	}
}

// Без сопоставимого периода прошлого года сравнение не показывается вовсе:
// «−100%» читалось бы как обвал продаж, а не как отсутствие истории.
func TestAggregateNetworkDashboardHidesYoYWithoutHistory(t *testing.T) {
	testCase := dashboardCase{
		networks: []models.Network{dashboardNetwork(1, "Аптека Плюс", "Иванов")},
		plans: []models.NetworkPlan{
			{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(1000000)},
		},
		facts: []models.NetworkMonthlyFact{dashboardFact(1, 1, "Альфа", 600000, 0)},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1), dashboardNow)

	if got.Summary.PrevFactRub != nil || got.Summary.FactYoYPct != nil {
		t.Errorf("сравнение с прошлым годом = %v / %v, ожидалось пусто",
			got.Summary.PrevFactRub, got.Summary.FactYoYPct)
	}
}

// Метки промо склеиваются по коду и каналу, а счётчики раскладываются
// на онлайн и оффлайн по справочнику механик.
func TestAggregateNetworkDashboardBuildsPromoTags(t *testing.T) {
	online, offline := "онлайн", "оффлайн"
	discount, ecomDiscount, bundle := "Скидка", "e-comm скидка", "Бандл"

	testCase := dashboardCase{
		networks: []models.Network{dashboardNetwork(1, "Аптека Плюс", "Иванов")},
		plans: []models.NetworkPlan{
			{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(1000000)},
		},
		promos: []repository.NetworkDashboardPromoRow{
			{NetworkName: "Аптека Плюс", Year: 2026, Month: 2, Mechanics: &discount, Channel: &offline, PromoCount: 3, PlanRub: 300},
			{NetworkName: "Аптека Плюс", Year: 2026, Month: 2, Mechanics: &ecomDiscount, Channel: &online, PromoCount: 5, PlanRub: 500},
			{NetworkName: "Аптека Плюс", Year: 2026, Month: 2, Mechanics: &bundle, Channel: &offline, PromoCount: 1, PlanRub: 100},
		},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1), dashboardNow)

	if got.Summary.PromoCount != 9 {
		t.Errorf("промо = %d, ожидалось 9", got.Summary.PromoCount)
	}
	if got.Summary.PromoOnlineCount != 5 || got.Summary.PromoOfflineCount != 4 {
		t.Errorf("каналы промо = %d онлайн / %d оффлайн, ожидалось 5 / 4",
			got.Summary.PromoOnlineCount, got.Summary.PromoOfflineCount)
	}

	if len(got.NetworkQuarters) != 1 {
		t.Fatalf("ячеек = %d, ожидалась 1", len(got.NetworkQuarters))
	}
	tags := got.NetworkQuarters[0].PromoTags
	if len(tags) != 3 {
		t.Fatalf("меток = %d, ожидалось 3: %+v", len(tags), tags)
	}
	// Сортировка по количеству: самая частая механика первой.
	if tags[0].Code != "СКИД" || tags[0].Channel != "онлайн" || tags[0].Count != 5 {
		t.Errorf("первая метка = %+v, ожидалось СКИД/онлайн/5", tags[0])
	}
	// «e-comm скидка» и «Скидка» — одна механика в разных каналах, поэтому код
	// у них общий, а различает их канал.
	if tags[1].Code != "СКИД" || tags[1].Channel != "оффлайн" {
		t.Errorf("вторая метка = %+v, ожидалось СКИД/оффлайн", tags[1])
	}
}

func TestPromoShortCode(t *testing.T) {
	cases := map[string]string{
		"Скидка":                "СКИД",
		"e-comm скидка":         "СКИД",
		"pure бандлы":           "БАНД",
		"УСТМ":                  "УСТМ",
		"УСТМ в ОС":             "УСТМ·В",
		"Амбассадоры в ОС":      "АМБА·В",
		"e-comm подсветка mark": "ПОДС·M",
		"":                      "—",
	}
	for input, want := range cases {
		if got := promoShortCode(input); got != want {
			t.Errorf("promoShortCode(%q) = %q, ожидалось %q", input, got, want)
		}
	}
}

// Главный инвариант витрины: её числа по сети должны совпадать с квартальными
// итогами карточки, посчитанными по тем же строкам.
func TestAggregateNetworkDashboardMatchesCardTotals(t *testing.T) {
	network := dashboardNetwork(1, "Аптека Плюс", "Иванов")
	plans := []models.NetworkPlan{
		{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: nil, PlanRub: models.PtrFloat(1000000)},
		{
			NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"), InGross: true,
			PlanRub: models.PtrFloat(700000), InvestmentsPct: models.PtrFloat(8),
		},
		{
			NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Бета"),
			PlanRub: models.PtrFloat(300000), InvestmentsPct: models.PtrFloat(12),
		},
	}
	facts := []models.NetworkMonthlyFact{
		dashboardFact(1, 1, "Альфа", 200000, 16000),
		dashboardFact(1, 2, "Альфа", 250000, 20000),
		dashboardFact(1, 3, "Бета", 150000, 18000),
	}

	testCase := dashboardCase{networks: []models.Network{network}, plans: plans, facts: facts}
	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1), dashboardNow)

	// Тот же расчёт «руками», как его делает карточка сети.
	slice := buildNetworkSlice(network, 2026, map[int]bool{1: true}, plans, nil, facts, nil, dashboardNow)
	want := slice.quarterTotals[1]

	if got.Summary.PlanRub != want.ContractPlanRub {
		t.Errorf("план витрины = %v, карточка = %v", got.Summary.PlanRub, want.ContractPlanRub)
	}
	if got.Summary.FactRub != want.FactRub {
		t.Errorf("факт витрины = %v, карточка = %v", got.Summary.FactRub, want.FactRub)
	}
	if got.Summary.EACRub != want.ForecastRub {
		t.Errorf("EAC витрины = %v, карточка = %v", got.Summary.EACRub, want.ForecastRub)
	}
	if got.Summary.PlanInvestmentsRubNet != want.InvestmentsRubNet {
		t.Errorf("чистые плановые инвестиции витрины = %v, карточка = %v",
			got.Summary.PlanInvestmentsRubNet, want.InvestmentsRubNet)
	}
	if got.Summary.FactInvestmentsRubNet != want.FactInvestmentsRubNet {
		t.Errorf("чистый факт инвестиций витрины = %v, карточка = %v",
			got.Summary.FactInvestmentsRubNet, want.FactInvestmentsRubNet)
	}
}

// Разрезы по сетям, брендам и КАМам собираются из одних и тех же строк.
func TestAggregateNetworkDashboardBuildsBreakdowns(t *testing.T) {
	testCase := dashboardCase{
		networks: []models.Network{
			dashboardNetwork(1, "Аптека Плюс", "Иванов"),
			dashboardNetwork(2, "Здоровье", "Иванов"),
			dashboardNetwork(3, "Мегафарм", "Петров"),
		},
		plans: []models.NetworkPlan{
			{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(500000)},
			{NetworkID: 2, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(300000)},
			{NetworkID: 3, Year: 2026, Quarter: 1, BrandAS: brandPtr("Бета"), PlanRub: models.PtrFloat(900000)},
		},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1), dashboardNow)

	if len(got.Networks) != 3 || len(got.Brands) != 2 || len(got.KAMs) != 2 {
		t.Fatalf("разрезы = %d сетей, %d брендов, %d КАМов; ожидалось 3, 2, 2",
			len(got.Networks), len(got.Brands), len(got.KAMs))
	}
	// Порядок — по плановому объёму, чтобы крупное было сверху.
	if got.Networks[0].Name != "Мегафарм" {
		t.Errorf("первая сеть = %s, ожидалась Мегафарм", got.Networks[0].Name)
	}
	if got.Networks[0].NetworkID == nil || *got.Networks[0].NetworkID != 3 {
		t.Errorf("id сети для перехода в карточку = %v, ожидался 3", got.Networks[0].NetworkID)
	}
	kamPlan := map[string]float64{}
	for _, kam := range got.KAMs {
		kamPlan[kam.Name] = kam.Metrics.PlanRub
	}
	if kamPlan["Иванов"] != 800000 || kamPlan["Петров"] != 900000 {
		t.Errorf("планы по КАМам = %v, ожидалось Иванов 800000 и Петров 900000", kamPlan)
	}
	if got.Summary.NetworkCount != 3 || got.Summary.BrandCount != 2 {
		t.Errorf("итог = %d сетей и %d брендов, ожидалось 3 и 2",
			got.Summary.NetworkCount, got.Summary.BrandCount)
	}
	if len(got.NetworkQuarters) != 3 {
		t.Errorf("ячеек тепловой карты = %d, ожидалось 3", len(got.NetworkQuarters))
	}
}

// Бренд, который ведут и по SKU, и строкой бренда, не должен удваиваться:
// готовая строка бренда важнее, SKU складываются только при её отсутствии.
func TestAggregateForecastLinesPrefersBrandRow(t *testing.T) {
	lines := []models.NetworkForecastLine{
		{Year: 2026, Month: 5, BrandAS: "Альфа", ForecastRub: models.PtrFloat(500000)},
		{Year: 2026, Month: 5, BrandAS: "Альфа", SKU: models.PtrString("SKU-1"), ForecastRub: models.PtrFloat(300000)},
		{Year: 2026, Month: 5, BrandAS: "Альфа", SKU: models.PtrString("SKU-2"), ForecastRub: models.PtrFloat(400000)},
		{Year: 2026, Month: 5, BrandAS: "Бета", SKU: models.PtrString("SKU-3"), ForecastRub: models.PtrFloat(120000)},
		{Year: 2026, Month: 5, BrandAS: "Бета", SKU: models.PtrString("SKU-4"), ForecastRub: models.PtrFloat(80000)},
	}

	got := aggregateForecastLines(lines)

	alpha := got[forecastMonthKey(2026, 5, "Альфа")]
	if alpha.rub == nil || *alpha.rub != 500000 {
		t.Errorf("прогноз Альфы = %v, ожидалось 500000 из строки бренда", alpha.rub)
	}
	beta := got[forecastMonthKey(2026, 5, "Бета")]
	if beta.rub == nil || *beta.rub != 200000 {
		t.Errorf("прогноз Беты = %v, ожидалось 200000 суммой SKU", beta.rub)
	}
}

// Сеть без закреплённого КАМа собирается в отдельную строку, а не растворяется
// в пустом имени.
func TestAggregateNetworkDashboardGroupsNetworksWithoutKAM(t *testing.T) {
	orphan := dashboardNetwork(1, "Ничейная", "")
	orphan.KAM = nil

	testCase := dashboardCase{
		networks: []models.Network{orphan},
		plans: []models.NetworkPlan{
			{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(100000)},
		},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1), dashboardNow)

	if len(got.KAMs) != 1 || got.KAMs[0].Name != networkUnassignedKAM {
		t.Errorf("разрез по КАМам = %+v, ожидалась одна строка «%s»", got.KAMs, networkUnassignedKAM)
	}
}

// Код из справочника важнее вычисленного: сокращения согласованы людьми и
// попадают в презентации, поэтому автоматическое правило их не переписывает.
func TestAggregateNetworkDashboardPrefersAssignedPromoCode(t *testing.T) {
	online := "онлайн"
	mechanics := "e-comm подсветка"
	assigned := "@-СВ"

	testCase := dashboardCase{
		networks: []models.Network{dashboardNetwork(1, "Аптека Плюс", "Иванов")},
		plans: []models.NetworkPlan{
			{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(1000000)},
		},
		promos: []repository.NetworkDashboardPromoRow{{
			NetworkName: "Аптека Плюс", Year: 2026, Month: 2,
			Mechanics: &mechanics, Channel: &online, ShortCode: &assigned,
			PromoCount: 2, PlanRub: 200,
		}},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1), dashboardNow)

	tags := got.NetworkQuarters[0].PromoTags
	if len(tags) != 1 || tags[0].Code != "@-СВ" {
		t.Fatalf("метки = %+v, ожидался назначенный код @-СВ", tags)
	}
	if tags[0].Mechanics != mechanics {
		t.Errorf("название механики = %q, ожидалось %q", tags[0].Mechanics, mechanics)
	}
}

// Механика без назначенного кода не должна ломать плитку: сокращение считается
// на лету, чтобы новая механика в рабочей базе не требовала миграции.
func TestAggregateNetworkDashboardFallsBackToComputedPromoCode(t *testing.T) {
	offline := "оффлайн"
	mechanics := "Новая механика"

	testCase := dashboardCase{
		networks: []models.Network{dashboardNetwork(1, "Аптека Плюс", "Иванов")},
		plans: []models.NetworkPlan{
			{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(1000000)},
		},
		promos: []repository.NetworkDashboardPromoRow{{
			NetworkName: "Аптека Плюс", Year: 2026, Month: 2,
			Mechanics: &mechanics, Channel: &offline, ShortCode: nil,
			PromoCount: 1, PlanRub: 100,
		}},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1), dashboardNow)

	tags := got.NetworkQuarters[0].PromoTags
	if len(tags) != 1 || tags[0].Code != promoShortCode(mechanics) {
		t.Fatalf("метки = %+v, ожидалось запасное сокращение %q", tags, promoShortCode(mechanics))
	}
}

// Пустая строка в справочнике — это не код: она означает «не назначен».
func TestPromoCodeOfTreatsBlankAsUnassigned(t *testing.T) {
	blank := "   "
	if got := promoCodeOf(&blank, "Скидка"); got != promoShortCode("Скидка") {
		t.Errorf("promoCodeOf(пробелы) = %q, ожидалось запасное сокращение", got)
	}
	assigned := "СК"
	if got := promoCodeOf(&assigned, "Скидка"); got != "СК" {
		t.Errorf("promoCodeOf(СК) = %q, ожидалось СК", got)
	}
}

// Месячный ряд: план раскладывается схемой из профиля сети, факт и EAC
// приходят из помесячных строк, а сумма месяцев сходится с кварталом.
func TestAggregateNetworkDashboardBuildsMonthlyTrend(t *testing.T) {
	network := dashboardNetwork(1, "Аптека Плюс", "Иванов")
	// 20 / 30 / 50 вместо умолчания — чтобы раскладка была видна в числах.
	network.Month1Pct, network.Month2Pct, network.Month3Pct = 20, 30, 50

	testCase := dashboardCase{
		networks: []models.Network{network},
		plans: []models.NetworkPlan{{
			NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"),
			PlanRub: models.PtrFloat(1000000), InvestmentsPct: models.PtrFloat(10),
		}},
		facts: []models.NetworkMonthlyFact{
			dashboardFact(1, 1, "Альфа", 300000, 0),
			dashboardFact(1, 3, "Альфа", 200000, 0),
		},
		prevFacts: []models.NetworkMonthlyFact{{
			NetworkID: 1, Year: 2025, Month: 1, BrandAS: "Альфа",
			SKU: models.PtrString("SKU-1"), FactRub: models.PtrFloat(250000),
		}},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1), dashboardNow)

	if len(got.Months) != 3 {
		t.Fatalf("месяцев = %d, ожидалось 3: %+v", len(got.Months), got.Months)
	}
	if got.Months[0].Month != 1 || got.Months[2].Month != 3 {
		t.Errorf("порядок месяцев = %d..%d, ожидалось 1..3", got.Months[0].Month, got.Months[2].Month)
	}

	// План квартала 1 000 000 разложен как 20 / 30 / 50.
	wantPlan := []float64{200000, 300000, 500000}
	for i, want := range wantPlan {
		if got.Months[i].PlanRub != want {
			t.Errorf("план месяца %d = %v, ожидалось %v", i+1, got.Months[i].PlanRub, want)
		}
	}

	// Февраль без отгрузок — это ноль, а не пропуск: точка на графике нужна.
	if got.Months[1].FactRub != 0 {
		t.Errorf("факт февраля = %v, ожидался 0", got.Months[1].FactRub)
	}

	// Сумма месяцев сходится с кварталом и с итогом.
	var factSum, planSum float64
	for _, month := range got.Months {
		factSum = round2(factSum + month.FactRub)
		planSum = round2(planSum + month.PlanRub)
	}
	if factSum != got.Summary.FactRub {
		t.Errorf("сумма месяцев по факту = %v, итог витрины = %v", factSum, got.Summary.FactRub)
	}
	if planSum != got.Summary.PlanRub {
		t.Errorf("сумма месяцев по плану = %v, итог витрины = %v", planSum, got.Summary.PlanRub)
	}

	// Прошлый год приходит помесячно из тех же строк факта.
	if got.Months[0].PrevFactRub == nil || *got.Months[0].PrevFactRub != 250000 {
		t.Errorf("прошлый год за январь = %v, ожидалось 250000", got.Months[0].PrevFactRub)
	}
	if got.Months[1].PrevFactRub != nil {
		t.Errorf("прошлый год за февраль = %v, ожидалось пусто", got.Months[1].PrevFactRub)
	}

	// Закрытость месяца решает дата витрины, а не календарь машины.
	if !got.Months[0].Closed || !got.Months[2].Closed {
		t.Errorf("месяцы Q1 должны быть закрыты на 15 мая 2026")
	}
}

// Промо считаются помесячно, а квартал складывается из месяцев.
func TestAggregateNetworkDashboardCountsPromoByMonth(t *testing.T) {
	offline := "оффлайн"
	discount := "Скидка"

	testCase := dashboardCase{
		networks: []models.Network{dashboardNetwork(1, "Аптека Плюс", "Иванов")},
		plans: []models.NetworkPlan{{
			NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"),
			PlanRub: models.PtrFloat(1000000),
		}},
		promos: []repository.NetworkDashboardPromoRow{
			{NetworkName: "Аптека Плюс", Year: 2026, Month: 1, Mechanics: &discount, Channel: &offline, PromoCount: 2},
			{NetworkName: "Аптека Плюс", Year: 2026, Month: 3, Mechanics: &discount, Channel: &offline, PromoCount: 5},
		},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1), dashboardNow)

	byMonth := map[int]int{}
	for _, month := range got.Months {
		byMonth[month.Month] = month.PromoCount
	}
	if byMonth[1] != 2 || byMonth[2] != 0 || byMonth[3] != 5 {
		t.Errorf("промо по месяцам = %v, ожидалось 2 / 0 / 5", byMonth)
	}
	if got.Summary.PromoCount != 7 {
		t.Errorf("промо за квартал = %d, ожидалось 7", got.Summary.PromoCount)
	}
}

// Несмежные кварталы: сравнить Q1 с Q3 диапазоном нельзя, и ради этого набор
// и заменил границы «от и до».
func TestAggregateNetworkDashboardSupportsNonAdjacentQuarters(t *testing.T) {
	plan := func(quarter int, rub float64) models.NetworkPlan {
		return models.NetworkPlan{
			NetworkID: 1, Year: 2026, Quarter: quarter, BrandAS: brandPtr("Альфа"),
			PlanRub: models.PtrFloat(rub),
		}
	}
	testCase := dashboardCase{
		networks: []models.Network{dashboardNetwork(1, "Аптека Плюс", "Иванов")},
		plans:    []models.NetworkPlan{plan(1, 100000), plan(2, 200000), plan(3, 300000), plan(4, 400000)},
		facts: []models.NetworkMonthlyFact{
			dashboardFact(1, 2, "Альфа", 90000, 0),  // Q1
			dashboardFact(1, 5, "Альфа", 190000, 0), // Q2 — не выбран
			dashboardFact(1, 8, "Альфа", 280000, 0), // Q3
		},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1, 3), dashboardNow)

	if len(got.SelectedQuarters) != 2 || got.SelectedQuarters[0] != 1 || got.SelectedQuarters[1] != 3 {
		t.Fatalf("выбранные кварталы = %v, ожидалось [1 3]", got.SelectedQuarters)
	}
	// Q2 не должен попасть ни в план, ни в факт.
	if got.Summary.PlanRub != 400000 {
		t.Errorf("план = %v, ожидалось 400000 (Q1 + Q3)", got.Summary.PlanRub)
	}
	if got.Summary.FactRub != 370000 {
		t.Errorf("факт = %v, ожидалось 370000 (Q1 + Q3)", got.Summary.FactRub)
	}

	quarters := []int{}
	for _, point := range got.Quarters {
		quarters = append(quarters, point.Quarter)
	}
	if len(quarters) != 2 || quarters[0] != 1 || quarters[1] != 3 {
		t.Errorf("точки тренда = %v, ожидалось [1 3]", quarters)
	}

	// Месяцы приходят только выбранных кварталов, без апреля-июня.
	months := []int{}
	for _, point := range got.Months {
		months = append(months, point.Month)
	}
	want := []int{1, 2, 3, 7, 8, 9}
	if len(months) != len(want) {
		t.Fatalf("месяцы = %v, ожидалось %v", months, want)
	}
	for i, value := range want {
		if months[i] != value {
			t.Errorf("месяцы = %v, ожидалось %v", months, want)
			break
		}
	}
}

// Пустой выбор — это весь год, а не пустая витрина.
func TestNormalizedQuartersDefaultsToWholeYear(t *testing.T) {
	empty := repository.NetworkDashboardFilter{Year: 2026}
	if got := empty.NormalizedQuarters(); len(got) != 4 {
		t.Errorf("пустой выбор = %v, ожидался весь год", got)
	}
	// Повторы и мусор отбрасываются, порядок восстанавливается.
	messy := repository.NetworkDashboardFilter{Year: 2026, Quarters: []int{3, 1, 3, 0, 9}}
	got := messy.NormalizedQuarters()
	if len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Errorf("нормализация = %v, ожидалось [1 3]", got)
	}
	months := messy.Months()
	if len(months) != 6 || months[0] != 1 || months[5] != 9 {
		t.Errorf("месяцы = %v, ожидалось [1 2 3 7 8 9]", months)
	}
}

// ─── Разбор одной сети ──────────────────────────────────────────────────────
//
// Разрезы «бренд × квартал» и промо-календарь заполняются только когда в
// области ровно одна сеть: на портфеле это произведение размерностей.

func TestBrandQuartersOnlyForSingleNetworkScope(t *testing.T) {
	twoNetworks := dashboardCase{
		networks: []models.Network{
			dashboardNetwork(1, "Аптека Плюс", "Иванов"),
			dashboardNetwork(2, "Аптека Минус", "Петров"),
		},
		plans: []models.NetworkPlan{
			{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(1000)},
			{NetworkID: 2, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(2000)},
		},
		facts: []models.NetworkMonthlyFact{
			dashboardFact(1, 1, "Альфа", 900, 90),
			dashboardFact(2, 1, "Альфа", 1800, 180),
		},
	}

	got := AggregateNetworkDashboard(twoNetworks.data(), dashboardFilter(1), dashboardNow)
	if len(got.BrandQuarters) != 0 {
		t.Fatalf("на портфеле разрез отдан: %d строк, ожидалось 0", len(got.BrandQuarters))
	}
}

func TestBrandQuartersSplitBrandByQuarter(t *testing.T) {
	testCase := dashboardCase{
		networks: []models.Network{dashboardNetwork(1, "Аптека Плюс", "Иванов")},
		plans: []models.NetworkPlan{
			{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"),
				PlanRub: models.PtrFloat(1000), InvestmentsPct: models.PtrFloat(10)},
			{NetworkID: 1, Year: 2026, Quarter: 2, BrandAS: brandPtr("Альфа"),
				PlanRub: models.PtrFloat(2000), InvestmentsPct: models.PtrFloat(10)},
			{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Бета"),
				PlanRub: models.PtrFloat(500), InvestmentsPct: models.PtrFloat(10)},
		},
		facts: []models.NetworkMonthlyFact{
			dashboardFact(1, 1, "Альфа", 400, 40),
			dashboardFact(1, 2, "Альфа", 400, 40),
			dashboardFact(1, 4, "Альфа", 900, 90),
			dashboardFact(1, 1, "Бета", 600, 60),
		},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1, 2), dashboardNow)

	if len(got.BrandQuarters) != 3 {
		t.Fatalf("строк разреза = %d, ожидалось 3", len(got.BrandQuarters))
	}
	// Порядок устойчив: сначала бренд, потом квартал.
	if got.BrandQuarters[0].Brand != "Альфа" || got.BrandQuarters[0].Quarter != 1 {
		t.Errorf("первая строка = %s Q%d, ожидалось Альфа Q1",
			got.BrandQuarters[0].Brand, got.BrandQuarters[0].Quarter)
	}
	if got.BrandQuarters[2].Brand != "Бета" {
		t.Errorf("последняя строка = %s, ожидался Бета", got.BrandQuarters[2].Brand)
	}

	// Кварталы одного бренда не смешиваются между собой.
	alphaQ1 := got.BrandQuarters[0]
	alphaQ2 := got.BrandQuarters[1]
	if alphaQ1.Metrics.PlanRub != 1000 || alphaQ2.Metrics.PlanRub != 2000 {
		t.Errorf("планы по кварталам = %v и %v, ожидались 1000 и 2000",
			alphaQ1.Metrics.PlanRub, alphaQ2.Metrics.PlanRub)
	}
	if alphaQ1.Metrics.FactRub != 800 || alphaQ2.Metrics.FactRub != 900 {
		t.Errorf("факт по кварталам = %v и %v, ожидались 800 и 900",
			alphaQ1.Metrics.FactRub, alphaQ2.Metrics.FactRub)
	}
}

// Разрез обязан сходиться с разрезом брендов: это те же строки плана, просто
// не свёрнутые по кварталам. Расхождение здесь означало бы вторую арифметику.
func TestBrandQuartersSumUpToBrandBreakdown(t *testing.T) {
	testCase := dashboardCase{
		networks: []models.Network{dashboardNetwork(1, "Аптека Плюс", "Иванов")},
		plans: []models.NetworkPlan{
			{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"),
				PlanRub: models.PtrFloat(1000), InvestmentsPct: models.PtrFloat(10)},
			{NetworkID: 1, Year: 2026, Quarter: 2, BrandAS: brandPtr("Альфа"),
				PlanRub: models.PtrFloat(2000), InvestmentsPct: models.PtrFloat(10)},
		},
		facts: []models.NetworkMonthlyFact{
			dashboardFact(1, 1, "Альфа", 400, 40),
			dashboardFact(1, 2, "Альфа", 400, 40),
			dashboardFact(1, 4, "Альфа", 900, 90),
		},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1, 2), dashboardNow)

	var brand *models.NetworkDashboardBreakdown
	for index := range got.Brands {
		if got.Brands[index].Name == "Альфа" {
			brand = &got.Brands[index]
		}
	}
	if brand == nil {
		t.Fatal("в разрезе брендов нет Альфы")
	}

	var planSum, factSum, eacSum float64
	for _, row := range got.BrandQuarters {
		if row.Brand != "Альфа" {
			continue
		}
		planSum += row.Metrics.PlanRub
		factSum += row.Metrics.FactRub
		eacSum += row.Metrics.EACRub
	}
	if planSum != brand.Metrics.PlanRub {
		t.Errorf("сумма планов по кварталам = %v, у бренда %v", planSum, brand.Metrics.PlanRub)
	}
	if factSum != brand.Metrics.FactRub {
		t.Errorf("сумма факта по кварталам = %v, у бренда %v", factSum, brand.Metrics.FactRub)
	}
	if eacSum != brand.Metrics.EACRub {
		t.Errorf("сумма EAC по кварталам = %v, у бренда %v", eacSum, brand.Metrics.EACRub)
	}
}

func TestBrandMonthsCarryPromoCalendar(t *testing.T) {
	discount := "Скидка 20%"
	offline := "оффлайн"
	online := "онлайн"
	testCase := dashboardCase{
		networks: []models.Network{dashboardNetwork(1, "Аптека Плюс", "Иванов")},
		plans: []models.NetworkPlan{
			{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(1000)},
		},
		facts: []models.NetworkMonthlyFact{dashboardFact(1, 1, "Альфа", 900, 90)},
		promos: []repository.NetworkDashboardPromoRow{
			{NetworkName: "Аптека Плюс", Year: 2026, Month: 2, BrandAS: brandPtr("Альфа"),
				Mechanics: &discount, Channel: &offline, PromoCount: 2, PlanRub: 200, InvestRub: 20},
			{NetworkName: "Аптека Плюс", Year: 2026, Month: 2, BrandAS: brandPtr("Альфа"),
				Mechanics: &discount, Channel: &online, PromoCount: 1, PlanRub: 100, InvestRub: 10},
			{NetworkName: "Аптека Плюс", Year: 2026, Month: 3, BrandAS: brandPtr("Бета"),
				Mechanics: &discount, Channel: &offline, PromoCount: 1, PlanRub: 50, InvestRub: 5},
		},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1), dashboardNow)

	if len(got.BrandMonths) != 2 {
		t.Fatalf("ячеек календаря = %d, ожидалось 2", len(got.BrandMonths))
	}
	alpha := got.BrandMonths[0]
	if alpha.Brand != "Альфа" || alpha.Month != 2 {
		t.Fatalf("первая ячейка = %s месяц %d, ожидалась Альфа месяц 2", alpha.Brand, alpha.Month)
	}
	if alpha.PromoCount != 3 || alpha.PromoOnlineCount != 1 || alpha.PromoOfflineCount != 2 {
		t.Errorf("счётчики = %d всего, %d онлайн, %d оффлайн; ожидались 3, 1, 2",
			alpha.PromoCount, alpha.PromoOnlineCount, alpha.PromoOfflineCount)
	}
	if alpha.PromoInvestmentsRub != 30 {
		t.Errorf("инвестиции промо = %v, ожидалось 30", alpha.PromoInvestmentsRub)
	}
	// Метки склеены по коду и каналу: одна механика в двух каналах — две метки.
	if len(alpha.PromoTags) != 2 {
		t.Errorf("меток = %d, ожидалось 2", len(alpha.PromoTags))
	}

	// Промо-календарь не выдумывает бренд там, где его в плане нет: строка
	// «Беты» приходит из промо и обязана дойти до календаря как есть.
	if got.BrandMonths[1].Brand != "Бета" || got.BrandMonths[1].PromoCount != 1 {
		t.Errorf("вторая ячейка = %s ×%d, ожидалась Бета ×1",
			got.BrandMonths[1].Brand, got.BrandMonths[1].PromoCount)
	}
}

// Промо, пришедшее без бренда, не должно оседать в календаре под пустым
// именем: в разрезе по месяцам оно по-прежнему учтено.
func TestBrandMonthsSkipPromoWithoutBrand(t *testing.T) {
	discount := "Скидка 20%"
	offline := "оффлайн"
	testCase := dashboardCase{
		networks: []models.Network{dashboardNetwork(1, "Аптека Плюс", "Иванов")},
		plans: []models.NetworkPlan{
			{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(1000)},
		},
		facts: []models.NetworkMonthlyFact{dashboardFact(1, 1, "Альфа", 900, 90)},
		promos: []repository.NetworkDashboardPromoRow{
			{NetworkName: "Аптека Плюс", Year: 2026, Month: 2, BrandAS: nil,
				Mechanics: &discount, Channel: &offline, PromoCount: 4, PlanRub: 400},
		},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1), dashboardNow)

	if len(got.BrandMonths) != 0 {
		t.Fatalf("ячеек календаря = %d, ожидалось 0", len(got.BrandMonths))
	}
	var monthPromo int
	for _, point := range got.Months {
		monthPromo += point.PromoCount
	}
	if monthPromo != 4 {
		t.Errorf("промо в месячном ряду = %d, ожидалось 4", monthPromo)
	}
}

// Ячейка без промо обязана нести пустой список меток, а не nil: nil уходит в
// JSON как null, сгенерированный тип обещает клиенту массив, и первое же
// обращение к длине роняет экран разбора.
func TestPromoTagsAreNeverNil(t *testing.T) {
	testCase := dashboardCase{
		networks: []models.Network{dashboardNetwork(1, "Аптека Плюс", "Иванов")},
		plans: []models.NetworkPlan{
			{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(1000)},
		},
		facts: []models.NetworkMonthlyFact{dashboardFact(1, 1, "Альфа", 900, 90)},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1), dashboardNow)

	for _, cell := range got.NetworkQuarters {
		if cell.PromoTags == nil {
			t.Errorf("ячейка %s Q%d без меток отдана как nil", cell.Name, cell.Quarter)
		}
	}
	if len(got.BrandQuarters) == 0 {
		t.Fatal("разрез «бренд × квартал» пуст")
	}
	for _, row := range got.BrandQuarters {
		if row.PromoTags == nil {
			t.Errorf("строка %s Q%d без меток отдана как nil", row.Brand, row.Quarter)
		}
	}

	// И то же самое в JSON: проверяется не поле, а то, что увидит клиент.
	encoded, err := json.Marshal(got.BrandQuarters[0])
	if err != nil {
		t.Fatalf("сериализация: %v", err)
	}
	if !strings.Contains(string(encoded), `"promoTags":[]`) {
		t.Errorf("в JSON нет пустого списка меток: %s", encoded)
	}
}

// Промо без механики не даёт меток, но счётчики промо обязаны остаться:
// ровно на этой паре ячейка и получала nil.
func TestPromoWithoutMechanicsKeepsCountsAndEmptyTags(t *testing.T) {
	offline := "оффлайн"
	testCase := dashboardCase{
		networks: []models.Network{dashboardNetwork(1, "Аптека Плюс", "Иванов")},
		plans: []models.NetworkPlan{
			{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(1000)},
		},
		facts: []models.NetworkMonthlyFact{dashboardFact(1, 1, "Альфа", 900, 90)},
		promos: []repository.NetworkDashboardPromoRow{{
			NetworkName: "Аптека Плюс", Year: 2026, Month: 2, BrandAS: brandPtr("Альфа"),
			Mechanics: nil, Channel: &offline, PromoCount: 2, PlanRub: 200,
		}},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1), dashboardNow)

	if len(got.BrandMonths) != 1 {
		t.Fatalf("ячеек календаря = %d, ожидалась 1", len(got.BrandMonths))
	}
	cell := got.BrandMonths[0]
	if cell.PromoCount != 2 {
		t.Errorf("промо = %d, ожидалось 2", cell.PromoCount)
	}
	if cell.PromoTags == nil {
		t.Fatal("метки отданы как nil")
	}
	if len(cell.PromoTags) != 0 {
		t.Errorf("меток = %d, ожидалось 0: механики у промо не было", len(cell.PromoTags))
	}
}

// ─── SKU внутри бренда ──────────────────────────────────────────────────────

func TestSKUsCarryFactWithoutPlan(t *testing.T) {
	testCase := dashboardCase{
		networks: []models.Network{dashboardNetwork(1, "Аптека Плюс", "Иванов")},
		plans: []models.NetworkPlan{
			{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"),
				PlanRub: models.PtrFloat(1000), InvestmentsPct: models.PtrFloat(10)},
		},
		facts: []models.NetworkMonthlyFact{
			skuFact(1, 1, "Альфа", "SKU-A", 600, 60),
			skuFact(1, 2, "Альфа", "SKU-B", 200, 20),
			skuFact(1, 3, "Альфа", "SKU-A", 100, 10),
		},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1), dashboardNow)

	if len(got.SKUs) != 2 {
		t.Fatalf("строк SKU = %d, ожидалось 2", len(got.SKUs))
	}
	// Внутри бренда — по убыванию ожидаемого объёма.
	if got.SKUs[0].SKU != "SKU-A" || got.SKUs[0].FactRub != 700 {
		t.Errorf("первая строка = %s на %v, ожидался SKU-A на 700", got.SKUs[0].SKU, got.SKUs[0].FactRub)
	}
	if got.SKUs[1].SKU != "SKU-B" || got.SKUs[1].FactRub != 200 {
		t.Errorf("вторая строка = %s на %v, ожидался SKU-B на 200", got.SKUs[1].SKU, got.SKUs[1].FactRub)
	}

	// Квартал закрыт целиком: сумма SKU обязана сойтись с брендом.
	var factSum float64
	for _, row := range got.SKUs {
		factSum += row.FactRub
	}
	var brandFact float64
	for _, brand := range got.Brands {
		if brand.Name == "Альфа" {
			brandFact = brand.Metrics.FactRub
		}
	}
	if factSum != brandFact {
		t.Errorf("сумма SKU = %v, у бренда %v", factSum, brandFact)
	}

	// Доли складываются в сто процентов: весь объём бренда объяснён SKU.
	var shareSum float64
	for _, row := range got.SKUs {
		if row.ShareOfBrandPct == nil {
			t.Fatalf("у %s нет доли бренда", row.SKU)
		}
		shareSum += *row.ShareOfBrandPct
	}
	if shareSum < 99.9 || shareSum > 100.1 {
		t.Errorf("сумма долей = %v, ожидалось 100", shareSum)
	}
}

// Прошлый год для SKU читается из факта, минуя строки плана: год с отгрузками
// без заведённых планов — обычное дело, и сравнение там нужнее всего.
func TestSKUsCompareWithPrevYearWithoutPrevPlans(t *testing.T) {
	testCase := dashboardCase{
		networks: []models.Network{dashboardNetwork(1, "Аптека Плюс", "Иванов")},
		plans: []models.NetworkPlan{
			{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(1000)},
		},
		facts: []models.NetworkMonthlyFact{skuFact(1, 1, "Альфа", "SKU-A", 800, 80)},
		// Планов за прошлый год нет вовсе — только отгрузки.
		prevFacts: []models.NetworkMonthlyFact{{
			NetworkID: 1, Year: 2025, Month: 1, BrandAS: "Альфа",
			SKU: models.PtrString("SKU-A"), FactRub: models.PtrFloat(400),
		}},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1), dashboardNow)

	if len(got.SKUs) != 1 {
		t.Fatalf("строк SKU = %d, ожидалась 1", len(got.SKUs))
	}
	row := got.SKUs[0]
	if row.PrevFactRub == nil || *row.PrevFactRub != 400 {
		t.Fatalf("прошлый год = %v, ожидалось 400", row.PrevFactRub)
	}
	if row.FactYoYPct == nil || *row.FactYoYPct != 100 {
		t.Errorf("прирост = %v, ожидалось 100", row.FactYoYPct)
	}
}

// SKU бренда, которого нет в плане, в разрез не попадают: такого бренда нет
// и в разрезе брендов, и строки повисли бы ни при чём.
func TestSKUsSkipBrandsWithoutPlan(t *testing.T) {
	testCase := dashboardCase{
		networks: []models.Network{dashboardNetwork(1, "Аптека Плюс", "Иванов")},
		plans: []models.NetworkPlan{
			{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(1000)},
		},
		facts: []models.NetworkMonthlyFact{
			skuFact(1, 1, "Альфа", "SKU-A", 800, 80),
			skuFact(1, 1, "Незапланированный", "SKU-X", 500, 50),
		},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1), dashboardNow)

	for _, row := range got.SKUs {
		if row.Brand != "Альфа" {
			t.Errorf("в разрезе есть SKU бренда %s вне плана", row.Brand)
		}
	}
}

func TestSKUsOnlyForSingleNetworkScope(t *testing.T) {
	testCase := dashboardCase{
		networks: []models.Network{
			dashboardNetwork(1, "Аптека Плюс", "Иванов"),
			dashboardNetwork(2, "Аптека Минус", "Петров"),
		},
		plans: []models.NetworkPlan{
			{NetworkID: 1, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(1000)},
			{NetworkID: 2, Year: 2026, Quarter: 1, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(2000)},
		},
		facts: []models.NetworkMonthlyFact{
			skuFact(1, 1, "Альфа", "SKU-A", 800, 80),
			skuFact(2, 1, "Альфа", "SKU-A", 1600, 160),
		},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(1), dashboardNow)

	if len(got.SKUs) != 0 {
		t.Fatalf("на портфеле разрез SKU отдан: %d строк, ожидалось 0", len(got.SKUs))
	}
}

// Открытый месяц без SKU-строки прогноза не достраивается: сумма SKU честно
// не дотягивает до бренда, у которого свой официальный прогноз.
func TestSKUEACIgnoresBrandLevelForecast(t *testing.T) {
	testCase := dashboardCase{
		networks: []models.Network{dashboardNetwork(1, "Аптека Плюс", "Иванов")},
		plans: []models.NetworkPlan{
			{NetworkID: 1, Year: 2026, Quarter: 2, BrandAS: brandPtr("Альфа"), PlanRub: models.PtrFloat(1000)},
		},
		facts: []models.NetworkMonthlyFact{skuFact(1, 4, "Альфа", "SKU-A", 300, 30)},
		// Май и июнь открыты; официальный прогноз заведён на бренде, без SKU.
		forecasts: []models.NetworkForecastLine{
			{NetworkID: 1, Year: 2026, Month: 5, BrandAS: "Альфа", ForecastRub: models.PtrFloat(400)},
			{NetworkID: 1, Year: 2026, Month: 6, BrandAS: "Альфа", ForecastRub: models.PtrFloat(500)},
		},
	}

	got := AggregateNetworkDashboard(testCase.data(), dashboardFilter(2), dashboardNow)

	if len(got.SKUs) != 1 {
		t.Fatalf("строк SKU = %d, ожидалась 1", len(got.SKUs))
	}
	// У SKU только апрельский факт: брендовый прогноз ему не принадлежит.
	if got.SKUs[0].EACRub != 300 {
		t.Errorf("EAC SKU = %v, ожидалось 300", got.SKUs[0].EACRub)
	}
	var brandEAC float64
	for _, brand := range got.Brands {
		if brand.Name == "Альфа" {
			brandEAC = brand.Metrics.EACRub
		}
	}
	if brandEAC != 1200 {
		t.Fatalf("EAC бренда = %v, ожидалось 1200", brandEAC)
	}
	// Доля показывает ровно ту часть, что объяснена SKU.
	if got.SKUs[0].ShareOfBrandPct == nil || *got.SKUs[0].ShareOfBrandPct != 25 {
		t.Errorf("доля = %v, ожидалось 25", got.SKUs[0].ShareOfBrandPct)
	}
}
