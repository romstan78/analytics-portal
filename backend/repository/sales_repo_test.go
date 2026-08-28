package repository

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildSalesWhereQuarters(t *testing.T) {
	where, args := BuildSalesWhere(SalesFilter{
		YearFromStr: "2026",
		Quarters:    []string{"2", "4", "0", "bad"},
	})

	if !strings.Contains(where, "((n.[month] - 1) / 3) + 1 IN (?,?)") {
		t.Fatalf("quarter condition is missing from %q", where)
	}
	wantArgs := []interface{}{2026, 2, 4}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestBuildSalesWhereExactFocus(t *testing.T) {
	where, args := BuildSalesWhere(SalesFilter{
		ProductExact: "SKU 1",
		NetworkExact: "Network 1",
	})

	if !strings.Contains(where, "n.productName = ?") || !strings.Contains(where, "n.networkName = ?") {
		t.Fatalf("focus conditions are missing from %q", where)
	}
	wantArgs := []interface{}{"SKU 1", "Network 1"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestBuildSalesWhereExactListsAndIn(t *testing.T) {
	where, args := BuildSalesWhere(SalesFilter{
		BrandNames: []string{"Альфа", ""},
		UnRubs:     []string{"руб", ""},
		Segments:   []string{"OLAP SS"},
	})

	if !strings.Contains(where, "n.brandName IN (?)") {
		t.Fatalf("brand IN condition is missing from %q", where)
	}
	if !strings.Contains(where, "n.un_rub IN (?)") || !strings.Contains(where, "n.segment IN (?)") {
		t.Fatalf("IN conditions are missing from %q", where)
	}
	wantArgs := []interface{}{"руб", "OLAP SS", "Альфа"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestBuildSalesWhereSearchRemainsPartial(t *testing.T) {
	where, args := BuildSalesWhere(SalesFilter{Search: "OLAP"})
	if !strings.Contains(where, "n.brandName LIKE ?") || !strings.Contains(where, "n.channel LIKE ?") {
		t.Fatalf("search conditions are missing from %q", where)
	}
	for _, arg := range args {
		if arg != "%OLAP%" {
			t.Fatalf("search arg = %#v, want %%OLAP%%", arg)
		}
	}
}

// Имя измерения подставляется в SQL как есть, поэтому список закрыт.
func TestCheckDimensionRejectsUnknownColumn(t *testing.T) {
	if err := checkDimension("networkName"); err != nil {
		t.Fatalf("checkDimension(networkName) = %v, want nil", err)
	}
	if err := checkDimension("metric_value); DROP TABLE"); err == nil {
		t.Fatal("checkDimension() accepted a column outside the whitelist")
	}
}

func TestPlaceholders(t *testing.T) {
	if got := placeholders(3); got != "?,?,?" {
		t.Fatalf("placeholders(3) = %q, want \"?,?,?\"", got)
	}
}

// orderColumns разбирает ORDER BY-константу на список колонок без направления сортировки.
func orderColumns(t *testing.T, order string) []string {
	t.Helper()

	const prefix = " ORDER BY "
	if !strings.HasPrefix(order, prefix) {
		t.Fatalf("порядок сортировки не начинается с %q: %q", prefix, order)
	}

	var cols []string
	for _, part := range strings.Split(strings.TrimPrefix(order, prefix), ",") {
		col := strings.TrimSpace(part)
		col = strings.TrimSuffix(col, " DESC")
		col = strings.TrimSuffix(col, " ASC")
		cols = append(cols, strings.TrimSpace(col))
	}
	return cols
}

// Без тай-брейка по первичному ключу строки с одинаковыми year/month/metric_type
// возвращаются в произвольном порядке, и OFFSET/FETCH в SalesRowsPage дублирует
// одни строки между страницами, теряя другие.
func TestSalesRowOrderHasPrimaryKeyTiebreak(t *testing.T) {
	cols := orderColumns(t, salesRowOrder)

	want := []string{"n.[year]", "n.[month]", "n.metric_type", "n.id"}
	if !reflect.DeepEqual(cols, want) {
		t.Fatalf("колонки ORDER BY = %v, want %v", cols, want)
	}
	if !strings.Contains(salesRowOrder, "n.[year] DESC") || !strings.Contains(salesRowOrder, "n.[month] ASC") {
		t.Fatalf("направление сортировки по году/месяцу изменилось: %q", salesRowOrder)
	}
}

func TestSalesRowOrderUsesWhitelistedSort(t *testing.T) {
	got := SalesRowOrder(SalesFilter{SortField: "metricValue", SortDirection: "desc"})
	if got != " ORDER BY n.metric_value DESC, n.id ASC" {
		t.Fatalf("SalesRowOrder() = %q", got)
	}

	got = SalesRowOrder(SalesFilter{SortField: "metric_value); DROP TABLE", SortDirection: "desc"})
	if got != salesRowOrder {
		t.Fatalf("unknown sort field did not fall back to default: %q", got)
	}
}

// В агрегирующем запросе drilldown первичный ключ недоступен, поэтому стабильность
// обеспечивают все колонки группировки — каждая из них обязана быть в GROUP BY,
// иначе SQL Server отвергнет запрос.
func TestSalesDrilldownOrderIsCoveredByGroupBy(t *testing.T) {
	if strings.Contains(salesDrilldownOrder, "n.id") {
		t.Fatalf("n.id не входит в GROUP BY и недопустим в ORDER BY drilldown: %q", salesDrilldownOrder)
	}

	const groupPrefix = " GROUP BY "
	if !strings.HasPrefix(salesDrilldownGroupBy, groupPrefix) {
		t.Fatalf("группировка не начинается с %q: %q", groupPrefix, salesDrilldownGroupBy)
	}

	grouped := make(map[string]bool)
	for _, part := range strings.Split(strings.TrimPrefix(salesDrilldownGroupBy, groupPrefix), ",") {
		grouped[strings.TrimSpace(part)] = true
	}

	ordered := orderColumns(t, salesDrilldownOrder)
	for _, col := range ordered {
		if !grouped[col] {
			t.Fatalf("колонка %q есть в ORDER BY, но отсутствует в GROUP BY %q", col, salesDrilldownGroupBy)
		}
	}

	// Тай-брейк полон только если сортировка покрывает всю группировку:
	// иначе у строк совпадут все ключи сортировки и порядок снова поплывёт.
	if len(ordered) != len(grouped) {
		t.Fatalf("ORDER BY покрывает %d из %d колонок GROUP BY: %q", len(ordered), len(grouped), salesDrilldownOrder)
	}
}

// Справочник обязан сужаться остальными фильтрами, но не своим собственным:
// иначе, выбрав бренд, пользователь увидел бы в списке брендов только его и не
// смог бы переключиться.
func TestSalesFilterOptionsExcludesOwnColumn(t *testing.T) {
	filter := SalesFilter{
		BrandNames:   []string{"Демо-бренд 06"},
		NetworkNames: []string{"Демо-сеть 31"},
		YearFromStr:  "2026",
	}

	withoutBrands := filter
	withoutBrands.BrandNames = nil
	brandWhere, brandArgs := BuildSalesWhere(withoutBrands)
	if strings.Contains(brandWhere, "brandName") {
		t.Fatalf("where для списка брендов = %q, свой фильтр применяться не должен", brandWhere)
	}
	// Остальные фильтры остаются: сеть и год сужают список брендов.
	if !strings.Contains(brandWhere, "networkName") || len(brandArgs) != 2 {
		t.Fatalf("where = %q args = %#v, ожидались сеть и год", brandWhere, brandArgs)
	}

	full, fullArgs := BuildSalesWhere(filter)
	if !strings.Contains(full, "brandName") || len(fullArgs) != 3 {
		t.Fatalf("полный фильтр потерял условия: %q %#v", full, fullArgs)
	}
}
