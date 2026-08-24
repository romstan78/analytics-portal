package repository

import (
	"errors"
	"strings"
	"testing"

	"backend/models"
)

func brandPtr(s string) *string { return &s }

func floatPtr(v float64) *float64 { return &v }

// Ключи строк, которые уйдут в запись, — для наглядного сравнения в тестах.
func writeKeys(rows []NetworkPlanInput) []string {
	keys := make([]string, 0, len(rows))
	for _, r := range rows {
		keys = append(keys, planKey(r.Quarter, r.BrandAS))
	}
	return keys
}

func TestPlanRowsToWriteKeepsIncomingRows(t *testing.T) {
	incoming := []NetworkPlanInput{
		{Quarter: 1, BrandAS: nil, PlanRub: floatPtr(100)},
		{Quarter: 1, BrandAS: brandPtr("Бренд А")},
	}

	write, remove := planRowsToWrite(incoming, nil)

	if got := writeKeys(write); len(got) != 2 || got[1] != "1|Бренд А" {
		t.Fatalf("write = %v, want обе пришедшие строки", got)
	}
	if len(remove) != 0 {
		t.Fatalf("remove = %v, want пусто", remove)
	}
}

func TestPlanRowsToWriteRemovesBrandMissingFromRequest(t *testing.T) {
	incoming := []NetworkPlanInput{{Quarter: 1, BrandAS: brandPtr("Бренд А")}}
	existing := []models.NetworkPlan{
		{ID: 1, Quarter: 1, BrandAS: brandPtr("Бренд А")},
		{ID: 2, Quarter: 1, BrandAS: brandPtr("Бренд Б"), PlanRub: floatPtr(500)},
		{ID: 3, Quarter: 1, BrandAS: nil, PlanRub: floatPtr(900)},
	}

	write, remove := planRowsToWrite(incoming, existing)

	if len(write) != 1 {
		t.Fatalf("write = %v, want только пришедшую строку", writeKeys(write))
	}
	if len(remove) != 1 || remove[0].ID != 2 {
		t.Fatalf("remove = %v, want строку бренда Б", remove)
	}
}

// Строку пула в запросе форма присылает всегда; сохранённый пул к удалению
// не относится, даже если его в запросе не оказалось.
func TestPlanRowsToWriteKeepsPoolRow(t *testing.T) {
	existing := []models.NetworkPlan{{ID: 1, Quarter: 2, BrandAS: nil, PlanRub: floatPtr(100)}}

	_, remove := planRowsToWrite([]NetworkPlanInput{{Quarter: 2, BrandAS: brandPtr("Бренд А")}}, existing)

	if len(remove) != 0 {
		t.Fatalf("remove = %v, want пусто", remove)
	}
}

// Факт приходит загрузкой отгрузок: такую строку очищаем, а не удаляем.
func TestPlanRowsToWriteClearsRowWithFactInsteadOfRemoving(t *testing.T) {
	existing := []models.NetworkPlan{
		{ID: 7, Quarter: 3, BrandAS: brandPtr("Бренд В"), PlanRub: floatPtr(300), FactRub: floatPtr(250), UpdatedAt: "2026-01-01 00:00:00"},
		{ID: 8, Quarter: 3, BrandAS: brandPtr("Бренд Г"), FactInvestmentsRub: floatPtr(40)},
	}

	write, remove := planRowsToWrite([]NetworkPlanInput{{Quarter: 3, BrandAS: nil}}, existing)

	if len(remove) != 0 {
		t.Fatalf("remove = %v, want пусто: строки с фактом не удаляются", remove)
	}
	if len(write) != 3 {
		t.Fatalf("write = %v, want пришедшую строку и две очищенные", writeKeys(write))
	}
	cleared := write[1]
	if cleared.PlanRub != nil || cleared.ForecastRub != nil || cleared.InvestmentsPct != nil || cleared.InGross {
		t.Fatalf("cleared = %#v, want пустые значения", cleared)
	}
	if cleared.UpdatedAt != "2026-01-01 00:00:00" {
		t.Fatalf("cleared.UpdatedAt = %q, want версию сохранённой строки", cleared.UpdatedAt)
	}
}

// Пустой запрос не переписывает год вслепую.
func TestPlanRowsToWriteIgnoresEmptyRequest(t *testing.T) {
	existing := []models.NetworkPlan{{ID: 1, Quarter: 1, BrandAS: brandPtr("Бренд А"), PlanRub: floatPtr(100)}}

	write, remove := planRowsToWrite(nil, existing)

	if len(write) != 0 || len(remove) != 0 {
		t.Fatalf("write = %v, remove = %v, want пусто", writeKeys(write), remove)
	}
}

// Входной срез не должен меняться: он же уходит в аудит и логи вызывающего.
func TestPlanRowsToWriteDoesNotMutateIncoming(t *testing.T) {
	incoming := make([]NetworkPlanInput, 1, 4)
	incoming[0] = NetworkPlanInput{Quarter: 1, BrandAS: brandPtr("Бренд А")}
	existing := []models.NetworkPlan{
		{ID: 2, Quarter: 1, BrandAS: brandPtr("Бренд Б"), FactRub: floatPtr(10)},
	}

	planRowsToWrite(incoming, existing)

	if len(incoming) != 1 {
		t.Fatalf("len(incoming) = %d, want 1", len(incoming))
	}
	if grown := incoming[:cap(incoming)]; grown[1].BrandAS != nil {
		t.Fatalf("запись ушла в буфер вызывающего: %#v", grown[1])
	}
}

func TestNormalizeNetworkPeriodGroupsAllowsDifferentBrands(t *testing.T) {
	groups := []NetworkPeriodGroupInput{
		{StartQuarter: 1, EndQuarter: 3, BrandAS: brandPtr("Бренд А")},
		{StartQuarter: 2, EndQuarter: 4, BrandAS: brandPtr("Бренд Б")},
	}
	allowed := map[string]bool{"Бренд А": true, "Бренд Б": true}

	got, err := NormalizeNetworkPeriodGroups(groups, allowed)
	if err != nil || len(got) != 2 {
		t.Fatalf("разные бренды должны объединяться независимо: got=%#v err=%v", got, err)
	}
}

func TestNormalizeNetworkPeriodGroupsRejectsPortfolioOverlap(t *testing.T) {
	groups := []NetworkPeriodGroupInput{
		{StartQuarter: 1, EndQuarter: 2},
		{StartQuarter: 2, EndQuarter: 4, BrandAS: brandPtr("Бренд А")},
	}

	_, err := NormalizeNetworkPeriodGroups(groups, map[string]bool{"Бренд А": true})
	if !errors.Is(err, ErrNetworkPeriodGroupInvalid) {
		t.Fatalf("пересечение портфеля и бренда: err=%v, ожидалась ErrNetworkPeriodGroupInvalid", err)
	}
}

func TestNormalizeNetworkPeriodGroupsRejectsSameBrandOverlap(t *testing.T) {
	groups := []NetworkPeriodGroupInput{
		{StartQuarter: 1, EndQuarter: 3, BrandAS: brandPtr("Бренд А")},
		{StartQuarter: 3, EndQuarter: 4, BrandAS: brandPtr("Бренд А")},
	}

	_, err := NormalizeNetworkPeriodGroups(groups, map[string]bool{"Бренд А": true})
	if !errors.Is(err, ErrNetworkPeriodGroupInvalid) {
		t.Fatalf("два зачёта одного квартала бренда: err=%v, ожидалась ErrNetworkPeriodGroupInvalid", err)
	}
}

func TestNormalizeNetworkPeriodGroupsRejectsSingleQuarterAndUnknownBrand(t *testing.T) {
	_, rangeErr := NormalizeNetworkPeriodGroups(
		[]NetworkPeriodGroupInput{{StartQuarter: 2, EndQuarter: 2}}, nil,
	)
	if !errors.Is(rangeErr, ErrNetworkPeriodGroupInvalid) {
		t.Fatalf("один квартал: err=%v, ожидалась ErrNetworkPeriodGroupInvalid", rangeErr)
	}

	_, brandErr := NormalizeNetworkPeriodGroups(
		[]NetworkPeriodGroupInput{{StartQuarter: 1, EndQuarter: 2, BrandAS: brandPtr("Нет в плане")}},
		map[string]bool{"Бренд А": true},
	)
	if !errors.Is(brandErr, ErrNetworkPeriodGroupInvalid) {
		t.Fatalf("неизвестный бренд: err=%v, ожидалась ErrNetworkPeriodGroupInvalid", brandErr)
	}
}

func TestGlobalOlapSSSKUPricesQueryDoesNotDependOnNetwork(t *testing.T) {
	if !strings.Contains(globalOlapSSSKUPricesQuery, "n.segment = N'OLAP SS'") {
		t.Fatal("запрос цены должен быть ограничен сегментом OLAP SS")
	}
	if strings.Contains(globalOlapSSSKUPricesQuery, "networkName") {
		t.Fatal("название сети не должно участвовать в расчёте цены SKU")
	}
	if !strings.Contains(globalOlapSSSKUPricesQuery, "n.un_rub = N'руб'") ||
		!strings.Contains(globalOlapSSSKUPricesQuery, "n.un_rub = N'уп'") {
		t.Fatal("цена должна рассчитываться как отношение рублей к упаковкам")
	}
}

func TestMergeNetworkContractPricesAddsSameSKUDefaultForAnyNetwork(t *testing.T) {
	defaults := []olapSKUPrice{{
		BrandAS: "Бренд А", SKU: "SKU-1", Price: 123.45, Year: 2026, Month: 7,
	}}

	got := mergeNetworkContractPrices(42, 2027, nil, defaults, nil)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	row := got[0]
	if row.NetworkID != 42 || row.ContractPrice != 123.45 || row.ValidFrom != "2027-01-01" || row.ValidTo != "2027-12-31" {
		t.Fatalf("default row = %#v", row)
	}
	if row.SourceType != "olap_seed" || row.SourceYear == nil || *row.SourceYear != 2026 || row.SourceMonth == nil || *row.SourceMonth != 7 {
		t.Fatalf("source = %#v", row)
	}
}

func TestMergeNetworkContractPricesRefreshesOnlyUnconfirmedSeed(t *testing.T) {
	sourceYear, sourceMonth := 2026, 3
	persisted := []models.NetworkContractPrice{
		{
			ID: 1, NetworkID: 8, BrandAS: "Старый бренд", SKU: "SKU-1", ContractPrice: 90,
			ValidFrom: "2026-01-01", ValidTo: "2026-12-31", SourceType: "olap_seed",
			SourceYear: &sourceYear, SourceMonth: &sourceMonth,
		},
		{
			ID: 2, NetworkID: 8, BrandAS: "Бренд Б", SKU: "SKU-2", ContractPrice: 77,
			ValidFrom: "2026-01-01", ValidTo: "2026-12-31", SourceType: "manual", IsConfirmed: true,
		},
	}
	defaults := []olapSKUPrice{
		{BrandAS: "Бренд А", SKU: "SKU-1", Price: 120, Year: 2026, Month: 7},
		{BrandAS: "Бренд Б", SKU: "SKU-2", Price: 130, Year: 2026, Month: 7},
	}

	got := mergeNetworkContractPrices(8, 2026, persisted, defaults, defaults)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 without duplicate defaults", len(got))
	}
	if got[0].SKU != "SKU-1" || got[0].ContractPrice != 120 || got[0].BrandAS != "Бренд А" || got[0].SourceMonth == nil || *got[0].SourceMonth != 7 {
		t.Fatalf("unconfirmed seed was not refreshed: %#v", got[0])
	}
	if got[1].SKU != "SKU-2" || got[1].ContractPrice != 77 {
		t.Fatalf("manual price was overwritten: %#v", got[1])
	}
	if got[1].OlapPrice == nil || *got[1].OlapPrice != 130 {
		t.Fatalf("OLAP SS comparison missing: %#v", got[1])
	}
}
