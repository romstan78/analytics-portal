package repository

import (
	"errors"
	"strings"
	"testing"
)

type promoScannerFunc func(dest ...interface{}) error

func (f promoScannerFunc) Scan(dest ...interface{}) error {
	return f(dest...)
}

func TestScanPromoRowUsesQueryColumnOrder(t *testing.T) {
	scanner := promoScannerFunc(func(dest ...interface{}) error {
		if got, want := len(dest), 49; got != want {
			t.Fatalf("destination count = %d, want %d", got, want)
		}

		network := "Антей АС"
		totalPharmacies := 101
		promoPharmacies := 55
		baselineUnits := 12.5
		agreement1 := "согласовано"
		agreement2 := "ожидает"
		channel := "Аптеки"

		*dest[0].(*int) = 321
		*dest[1].(**string) = &network
		*dest[16].(**int) = &totalPharmacies
		*dest[17].(**int) = &promoPharmacies
		*dest[18].(**float64) = &baselineUnits
		*dest[42].(**string) = &agreement1
		*dest[43].(**string) = &agreement2
		*dest[48].(**string) = &channel
		return nil
	})

	row, err := ScanPromoRow(scanner)
	if err != nil {
		t.Fatalf("ScanPromoRow() error = %v", err)
	}
	if row.ID != 321 || row.NetworkName == nil || *row.NetworkName != "Антей АС" {
		t.Fatalf("unexpected row identity: %+v", row)
	}
	if row.TotalPharmacies == nil || *row.TotalPharmacies != 101 {
		t.Fatalf("TotalPharmacies = %v, want 101", row.TotalPharmacies)
	}
	if row.PromoPharmacies == nil || *row.PromoPharmacies != 55 {
		t.Fatalf("PromoPharmacies = %v, want 55", row.PromoPharmacies)
	}
	if row.BaselineUnits == nil || *row.BaselineUnits != 12.5 {
		t.Fatalf("BaselineUnits = %v, want 12.5", row.BaselineUnits)
	}
	if row.Agreement1 == nil || *row.Agreement1 != "согласовано" {
		t.Fatalf("Agreement1 = %v, want согласовано", row.Agreement1)
	}
	if row.Agreement2 == nil || *row.Agreement2 != "ожидает" {
		t.Fatalf("Agreement2 = %v, want ожидает", row.Agreement2)
	}
	if row.PromoChannel == nil || *row.PromoChannel != "Аптеки" {
		t.Fatalf("PromoChannel = %v, want Аптеки", row.PromoChannel)
	}
}

func TestScanPromoRowReturnsScanError(t *testing.T) {
	wantErr := errors.New("scan failed")
	_, err := ScanPromoRow(promoScannerFunc(func(dest ...interface{}) error {
		return wantErr
	}))
	if !errors.Is(err, wantErr) {
		t.Fatalf("ScanPromoRow() error = %v, want %v", err, wantErr)
	}
}

func TestNormalizeBatchApproveItemsSortsAndRejectsDuplicates(t *testing.T) {
	items, err := normalizeBatchApproveItems([]BatchApproveItem{
		{ID: 20, UpdatedAt: "2026-08-16 01:00:00.000"},
		{ID: 10, UpdatedAt: "2026-08-16 01:00:00.000"},
	})
	if err != nil {
		t.Fatalf("normalizeBatchApproveItems() error = %v", err)
	}
	if items[0].ID != 10 || items[1].ID != 20 {
		t.Fatalf("items are not sorted: %+v", items)
	}

	_, err = normalizeBatchApproveItems([]BatchApproveItem{
		{ID: 10, UpdatedAt: "2026-08-16 01:00:00.000"},
		{ID: 10, UpdatedAt: "2026-08-16 01:00:00.000"},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate promo ID") {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestAppendApprovalCommentPreservesHistoryAndAuthor(t *testing.T) {
	got := appendApprovalComment("старый комментарий", "новый комментарий", "admin", 2)
	if !strings.HasPrefix(got, "старый комментарий\n[") {
		t.Fatalf("previous history was not preserved: %q", got)
	}
	if !strings.Contains(got, "согласование2|admin]: новый комментарий") {
		t.Fatalf("author or approval level is missing: %q", got)
	}
}

// Область согласования обязана попадать в WHERE независимо от того, что
// прислал клиент: именно она, а не параметр kam, ограничивает выборку.
func TestBuildApprovalsWhereAppliesScope(t *testing.T) {
	where, args := buildApprovalsWhere(ApprovalParams{
		Role:        "agreement2",
		AllowedKAMs: []string{"Алексеева Марина", "Крылов Сергей"},
		YearStr:     "2026",
	})
	if !strings.Contains(where, "p.kam IN (?,?)") {
		t.Fatalf("where = %q, ожидалось ограничение по области", where)
	}
	if len(args) != 3 || args[1] != "Алексеева Марина" || args[2] != "Крылов Сергей" {
		t.Fatalf("args = %#v, ожидались год и оба КАМа области", args)
	}
}

func TestBuildApprovalsWhereWithoutScopeStaysOpen(t *testing.T) {
	where, _ := buildApprovalsWhere(ApprovalParams{Role: "agreement1", YearStr: "2026"})
	if strings.Contains(where, "p.kam IN") {
		t.Fatalf("where = %q, без области ограничения быть не должно", where)
	}
}

// Клиентский фильтр по КАМу и область складываются: подстановка чужого КАМа
// не должна расширять выборку.
func TestBuildApprovalsWhereCombinesScopeWithRequestedKAM(t *testing.T) {
	where, args := buildApprovalsWhere(ApprovalParams{
		Role:        "agreement2",
		KAM:         "Ершов Максим",
		AllowedKAMs: []string{"Крылов Сергей"},
		YearStr:     "2026",
	})
	if !strings.Contains(where, "p.kam = ?") || !strings.Contains(where, "p.kam IN (?)") {
		t.Fatalf("where = %q, ожидались оба условия", where)
	}
	if len(args) != 3 {
		t.Fatalf("args = %#v, ожидались год, запрошенный КАМ и область", args)
	}
}

func TestBuildApprovalWhereFiltersScopeEvenForKAMColumn(t *testing.T) {
	// Справочник КАМов собирает саму колонку kam и потому исключает её из
	// фильтра. Область обязана применяться всё равно, иначе список выдал бы
	// имена КАМов вне зоны ответственности.
	where, args := buildApprovalWhere(ApprovalFilterParams{
		Role:        "agreement2",
		AllowedKAMs: []string{"Жукова Ольга"},
		YearStr:     "2026",
	}, "kam")
	if !strings.Contains(where, "p.kam IN (?)") {
		t.Fatalf("where = %q, область должна применяться и к колонке kam", where)
	}
	if args[len(args)-1] != "Жукова Ольга" {
		t.Fatalf("args = %#v, последним аргументом ожидался КАМ области", args)
	}
}

func TestKAMAllowedByScope(t *testing.T) {
	scope := []string{"Крылов Сергей", "Жукова Ольга"}
	if !KAMAllowedByScope(scope, "Крылов Сергей") {
		t.Fatal("КАМ области должен допускаться")
	}
	if KAMAllowedByScope(scope, "Ершов Максим") {
		t.Fatal("КАМ вне области не должен допускаться")
	}
	if !KAMAllowedByScope(nil, "Ершов Максим") {
		t.Fatal("пустая область не ограничивает")
	}
}

// Область видимости обязана попадать в базовое условие всех промо-выборок:
// справочники фильтров, строки, дашборд и выгрузка строятся на нём одном.
func TestBuildBaseWhereAppliesVisibilityScope(t *testing.T) {
	where, args := BuildBaseWhere(PromoFilterParams{
		AllowedKAMs: []string{"Ершов Максим", "Жукова Ольга"},
		YearFromStr: "2026",
	})
	if !strings.Contains(where, "kam IN (?,?)") {
		t.Fatalf("where = %q, ожидалось ограничение по области видимости", where)
	}
	if len(args) != 3 || args[0] != "Ершов Максим" || args[1] != "Жукова Ольга" {
		t.Fatalf("args = %#v, область должна идти первой", args)
	}
	if !strings.Contains(where, "deleted_at IS NULL") {
		t.Fatalf("where = %q, потеряно условие по удалённым", where)
	}
}

func TestBuildBaseWhereWithoutScopeStaysOpen(t *testing.T) {
	where, args := BuildBaseWhere(PromoFilterParams{YearFromStr: "2026"})
	if strings.Contains(where, "kam IN") {
		t.Fatalf("where = %q, без области ограничения быть не должно", where)
	}
	if len(args) != 1 {
		t.Fatalf("args = %#v, ожидался только год", args)
	}
}

// Строки, дашборд и выгрузка строятся вторым сборщиком условия — область
// обязана применяться и в нём, иначе ограничение осталось бы только на
// справочниках фильтров.
func TestBuildPromoWhereAppliesVisibilityScope(t *testing.T) {
	where, args := buildPromoWhere(PromoFilterParams{
		AllowedKAMs: []string{"Жукова Ольга"},
	}, nil)
	if !strings.Contains(where, "p.kam IN (?)") {
		t.Fatalf("where = %q, область не применена", where)
	}
	if len(args) != 1 || args[0] != "Жукова Ольга" {
		t.Fatalf("args = %#v, ожидалась область", args)
	}
}

// Клиентский фильтр по КАМу не расширяет область: условия складываются.
func TestBuildPromoWhereCombinesScopeWithRequestedKAMs(t *testing.T) {
	where, args := buildPromoWhere(PromoFilterParams{
		AllowedKAMs: []string{"Жукова Ольга"},
		Kams:        []string{"Ершов Максим"},
	}, nil)
	if strings.Count(where, "p.kam IN") != 2 {
		t.Fatalf("where = %q, ожидались оба условия по КАМу", where)
	}
	if len(args) != 2 || args[0] != "Жукова Ольга" || args[1] != "Ершов Максим" {
		t.Fatalf("args = %#v, область должна идти первой", args)
	}
}

func TestBuildPromoWhereWithoutScopeStaysOpen(t *testing.T) {
	where, _ := buildPromoWhere(PromoFilterParams{}, nil)
	if strings.Contains(where, "p.kam IN") {
		t.Fatalf("where = %q, без области ограничения быть не должно", where)
	}
}

// Нечисловой ввод не должен превращаться в ноль: «year >= 0» выглядит как
// применённый фильтр, а «month IN (0)» отдаёт пустую выдачу без объяснения.
func TestBuildBaseWhereIgnoresNonNumericPeriod(t *testing.T) {
	where, args := BuildBaseWhere(PromoFilterParams{
		YearFromStr: "abc",
		YearToStr:   "",
		Months:      []string{"abc"},
	})
	if strings.Contains(where, "year >=") || strings.Contains(where, "month IN") {
		t.Fatalf("where = %q, мусорный период не должен попадать в условие", where)
	}
	if len(args) != 0 {
		t.Fatalf("args = %#v, ожидался пустой набор", args)
	}
}

// Разбор периода обязан совпадать с витриной продаж: один и тот же запрос
// не может фильтроваться в промо иначе, чем в интернет-продажах.
func TestBuildBaseWhereMatchesSalesOnMixedMonths(t *testing.T) {
	where, args := BuildBaseWhere(PromoFilterParams{
		YearFromStr: "2026",
		Months:      []string{"3", "abc", "5"},
	})
	if !strings.Contains(where, "month IN (?,?)") {
		t.Fatalf("where = %q, ожидались только два разобранных месяца", where)
	}
	if len(args) != 3 || args[0] != 2026 || args[1] != 3 || args[2] != 5 {
		t.Fatalf("args = %#v, ожидались год и месяцы 3 и 5", args)
	}

	salesWhere, salesArgs := BuildSalesWhere(SalesFilter{
		YearFromStr: "2026",
		Months:      []string{"3", "abc", "5"},
	})
	if strings.Count(salesWhere, "?") != strings.Count(where, "?") || len(salesArgs) != len(args) {
		t.Fatalf("промо: %q %#v; продажи: %q %#v — разбор периода разошёлся",
			where, args, salesWhere, salesArgs)
	}
}

// Справочники согласования и сам список очереди обязаны читать период
// одинаково: иначе мусорный год оставлял бы карточки в списке, но обнулял
// выпадающие списки над ними.
func TestBuildApprovalWhereIgnoresNonNumericPeriod(t *testing.T) {
	where, args := buildApprovalWhere(ApprovalFilterParams{
		Role:     "agreement1",
		YearStr:  "abc",
		MonthStr: "abc",
	}, "")
	// Ищем именно отдельные условия: «p.year = ?» встречается и внутри ветки
	// по умолчанию «(p.year > ? OR (p.year = ? AND p.month >= ?))».
	if strings.Contains(where, " AND p.year = ?") || strings.Contains(where, " AND p.month = ?") {
		t.Fatalf("where = %q, мусорный период не должен попадать в условие", where)
	}
	// Год не разобран — очередь остаётся на поведении «без года»: будущее от
	// текущего месяца, а не пустая выдача.
	if !strings.Contains(where, "p.year > ?") || len(args) != 3 {
		t.Fatalf("where = %q args = %#v, ожидалось условие по умолчанию", where, args)
	}
}

func TestGetPromoHistoryQueryShapeMatchesFilters(t *testing.T) {
	// Разбор года в истории тот же, что в BuildBaseWhere: мусор отбрасывается.
	where, args := BuildBaseWhere(PromoFilterParams{YearFromStr: "abc", YearToStr: "2026"})
	if strings.Contains(where, "year >= ?") {
		t.Fatalf("where = %q, нечитаемый yearFrom не должен фильтровать", where)
	}
	if !strings.Contains(where, "year <= ?") || len(args) != 1 || args[0] != 2026 {
		t.Fatalf("where = %q args = %#v, ожидался только верхний год", where, args)
	}
}

// Идентификатор колонки нельзя подставить плейсхолдером, поэтому в ORDER BY
// попадает только имя из белого списка, а всё остальное — порядок по умолчанию.
func TestPromoRowsOrderByAllowsOnlyKnownColumns(t *testing.T) {
	if got := promoRowsOrderBy(PromoFilterParams{SortField: "sku", SortDirection: "desc"}); got != " ORDER BY p.sku DESC, p.id ASC" {
		t.Fatalf("order = %q", got)
	}
	// Тай-брейк по id обязателен: без него OFFSET/FETCH терял бы строки.
	if got := promoRowsOrderBy(PromoFilterParams{SortField: "year"}); got != " ORDER BY p.[year] ASC, p.id ASC" {
		t.Fatalf("order = %q", got)
	}
	for _, field := range []string{"", "unknown", "p.sku; DROP TABLE dbo.tbl_PromoActivities", "1=1"} {
		if got := promoRowsOrderBy(PromoFilterParams{SortField: field}); got != promoRowOrder {
			t.Fatalf("SortField=%q дал %q, ожидался порядок по умолчанию", field, got)
		}
	}
}

func TestEscapeLikePatternNeutralizesWildcards(t *testing.T) {
	// «%» в запросе должен искать сам символ, а не «что угодно».
	if got := escapeLikePattern("50%_[а]"); got != `50\%\_\[а]` {
		t.Fatalf("escapeLikePattern = %q", got)
	}
}

func TestBuildPromoWhereSearchesVisibleColumns(t *testing.T) {
	where, args := buildPromoWhere(PromoFilterParams{Search: "Север"}, nil)
	if !strings.Contains(where, "p.network_name LIKE ? ESCAPE") || !strings.Contains(where, "m.channel LIKE ?") {
		t.Fatalf("where = %q, поиск должен идти по видимым колонкам", where)
	}
	if len(args) != 9 {
		t.Fatalf("args = %#v, ожидался один шаблон на каждую колонку поиска", args)
	}
	if args[0] != "%Север%" {
		t.Fatalf("args[0] = %v, ожидался шаблон с обрамляющими процентами", args[0])
	}
}

func TestBuildPromoWhereIgnoresBlankSearch(t *testing.T) {
	where, args := buildPromoWhere(PromoFilterParams{Search: "   "}, nil)
	if strings.Contains(where, "LIKE") || len(args) != 0 {
		t.Fatalf("where = %q args = %#v, пустой поиск не должен попадать в условие", where, args)
	}
}

// Канал живёт в справочнике механик, а не в строке промо, поэтому сужает
// выборку подзапросом. Без него выбор канала не влиял ни на один другой список.
func TestChannelConditionLimitsByMechanics(t *testing.T) {
	cond, args := channelCondition([]string{"онлайн", "оффлайн"})
	if !strings.Contains(cond, "mechanics IN (SELECT mechanics FROM dbo.tbl_MechanicsChannelMapping") {
		t.Fatalf("условие = %q", cond)
	}
	if strings.Count(cond, "?") != 2 || len(args) != 2 {
		t.Fatalf("условие = %q args = %#v, ожидались два канала", cond, args)
	}
}

func TestChannelConditionIgnoresEmptyInput(t *testing.T) {
	for _, channels := range [][]string{nil, {}, {""}, {"  ", ""}} {
		cond, args := channelCondition(channels)
		if cond != "" || len(args) != 0 {
			t.Fatalf("channelCondition(%#v) = %q %#v, пустой выбор ничего не ограничивает", channels, cond, args)
		}
	}
}
