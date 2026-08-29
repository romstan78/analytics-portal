package models

// SKUReference — соответствие коммерческого SKU брендам в источнике и АС.
type SKUReference struct {
	ID        int    `json:"id"`
	SKU       string `json:"sku"`
	Brand     string `json:"brand"`
	BrandAS   string `json:"brand_as"`
	CreatedAt string `json:"created_at"`
}

// NetworkReference — атрибуты сети, используемые промо и аналитикой продаж.
type NetworkReference struct {
	ID           int    `json:"id"`
	NetworkName  string `json:"network_name"`
	KAM          string `json:"kam"`
	NetworkType  string `json:"network_type"`
	Top20Segment string `json:"top20_segment"`
	KeyRegion    string `json:"key_region"`
}

// KAMNetworkReference — период действия закрепления КАМ за сетью.
type KAMNetworkReference struct {
	ID          int    `json:"id"`
	KAM         string `json:"kam"`
	NetworkName string `json:"network_name"`
	ValidFrom   string `json:"valid_from"`
	CreatedAt   string `json:"created_at"`
}

// MechanicReference — механика промо и её аналитический канал.
type MechanicReference struct {
	ID        int    `json:"id"`
	Mechanics string `json:"mechanics"`
	Channel   string `json:"channel"`
	ShortCode string `json:"short_code"`
	CreatedAt string `json:"created_at"`
}

type DictionaryData struct {
	SKUs        []SKUReference        `json:"skus"`
	Networks    []NetworkReference    `json:"networks"`
	KAMNetworks []KAMNetworkReference `json:"kam_networks"`
	Mechanics   []MechanicReference   `json:"mechanics"`
}
