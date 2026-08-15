package repository

import (
	"errors"
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
