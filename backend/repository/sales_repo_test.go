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

func TestBuildSalesWhereLikeAndIn(t *testing.T) {
	where, args := BuildSalesWhere(SalesFilter{
		BrandNames: []string{"Альфа", ""},
		UnRubs:     []string{"руб", ""},
		Segments:   []string{"OLAP SS"},
	})

	if !strings.Contains(where, "n.brandName LIKE ?") {
		t.Fatalf("brand LIKE condition is missing from %q", where)
	}
	if !strings.Contains(where, "n.un_rub IN (?)") || !strings.Contains(where, "n.segment IN (?)") {
		t.Fatalf("IN conditions are missing from %q", where)
	}
	wantArgs := []interface{}{"%Альфа%", "руб", "OLAP SS"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
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
