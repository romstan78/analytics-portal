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
