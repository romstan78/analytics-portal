package repository

import "testing"

// Ключ приходит из чужого браузера и становится первичным ключом таблицы,
// поэтому проверяется формат, а не «непустая строка».
func TestNormalizePromoIdempotencyKey(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
		ok   bool
	}{
		{"обычный ключ", "3f2504e0-4f89-41d3-9a0c-0305e82c3301", "3f2504e0-4f89-41d3-9a0c-0305e82c3301", true},
		{"регистр не различается", "3F2504E0-4F89-41D3-9A0C-0305E82C3301", "3f2504e0-4f89-41d3-9a0c-0305e82c3301", true},
		{"пробелы по краям", "  3f2504e0-4f89-41d3-9a0c-0305e82c3301  ", "3f2504e0-4f89-41d3-9a0c-0305e82c3301", true},
		{"пустой", "", "", false},
		{"нет поля в запросе", "<nil>", "", false},
		{"короткий", "3f2504e0-4f89-41d3-9a0c-0305e82c33", "", false},
		{"не hex", "3f2504e0-4f89-41d3-9a0c-0305e82c33zz", "", false},
		{"дефисы не на месте", "3f2504e04-f89-41d3-9a0c-0305e82c3301", "", false},
		{"попытка подмешать SQL", "3f2504e0-4f89-41d3-9a0c-0305e82c33';--", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := NormalizePromoIdempotencyKey(tc.raw)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("NormalizePromoIdempotencyKey(%q) = (%q, %v), ожидалось (%q, %v)", tc.raw, got, ok, tc.want, tc.ok)
			}
		})
	}
}

// Один и тот же ключ в разном написании должен считаться одним: иначе повтор
// прошёл бы мимо уникального индекса и создал дубль.
func TestNormalizePromoIdempotencyKeyIsStable(t *testing.T) {
	upper, _ := NormalizePromoIdempotencyKey("3F2504E0-4F89-41D3-9A0C-0305E82C3301")
	lower, _ := NormalizePromoIdempotencyKey("3f2504e0-4f89-41d3-9a0c-0305e82c3301")
	if upper != lower {
		t.Fatalf("%q != %q", upper, lower)
	}
}
