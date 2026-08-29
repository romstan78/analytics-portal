package handlers

import (
	"strings"
	"testing"

	"backend/models"
)

func TestCleanSKUTrimsAndRequiresKey(t *testing.T) {
	input := models.SKUReference{SKU: "  SKU-01  ", Brand: "  Бренд  ", BrandAS: "  Бренд АС "}
	if message := cleanSKU(&input); message != "" {
		t.Fatalf("cleanSKU вернул ошибку: %s", message)
	}
	if input.SKU != "SKU-01" || input.Brand != "Бренд" || input.BrandAS != "Бренд АС" {
		t.Fatalf("значения не очищены: %#v", input)
	}

	empty := models.SKUReference{}
	if message := cleanSKU(&empty); !strings.Contains(message, "обязательное") {
		t.Fatalf("пустой SKU: ошибка = %q", message)
	}
}

func TestCleanKAMNetworkValidatesISODate(t *testing.T) {
	valid := models.KAMNetworkReference{KAM: " КАМ ", NetworkName: " Сеть ", ValidFrom: "2026-08-29"}
	if message := cleanKAMNetwork(&valid); message != "" {
		t.Fatalf("корректная дата отклонена: %s", message)
	}
	if valid.KAM != "КАМ" || valid.NetworkName != "Сеть" {
		t.Fatalf("значения не очищены: %#v", valid)
	}

	invalid := models.KAMNetworkReference{KAM: "КАМ", NetworkName: "Сеть", ValidFrom: "29.08.2026"}
	if message := cleanKAMNetwork(&invalid); !strings.Contains(message, "ГГГГ-ММ-ДД") {
		t.Fatalf("неверная дата: ошибка = %q", message)
	}
}

func TestCleanMechanicChecksShortCodeLength(t *testing.T) {
	input := models.MechanicReference{Mechanics: "Скидка", Channel: "offline", ShortCode: "TOO-LONG-CODE"}
	if message := cleanMechanic(&input); !strings.Contains(message, "длина") {
		t.Fatalf("длинный код: ошибка = %q", message)
	}
}
