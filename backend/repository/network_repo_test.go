package repository

import (
	"errors"
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
