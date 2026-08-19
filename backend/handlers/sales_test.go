package handlers

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildSalesWhereQuarters(t *testing.T) {
	where, args := buildSalesWhere(salesFilter{
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
	where, args := buildSalesWhere(salesFilter{
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

func TestUniqueNonEmptyStrings(t *testing.T) {
	got := uniqueNonEmptyStrings([]string{" A ", "", "B", "A", "C"}, 2)
	want := []string{"A", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("uniqueNonEmptyStrings() = %#v, want %#v", got, want)
	}
}
