// Генератор TypeScript-типов из Go-структур.
//
// Контракт API описан один раз — в Go. Файл frontend/src/types/api.generated.ts
// собирается отсюда, руками не правится, а CI сверяет его с исходниками
// (см. Makefile: make types-check). Если структура ответа изменилась, а типы
// не пересобраны, сборка падает — это и есть защита от рассинхронизации.
//
// Запуск: go run ./cmd/tsgen
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"backend/models"
	"backend/repository"
)

// entry — тип, попадающий в контракт. Name задаёт имя в TypeScript, если оно
// должно отличаться от имени Go-структуры.
type entry struct {
	value interface{}
	name  string
}

// exported — полный список типов контракта. Порядок определяет порядок в файле.
// Структура, на которую ссылается поле, обязана быть здесь же: генератор
// откажется собирать файл со ссылкой на неописанный тип.
var exported = []entry{
	// ─── Интернет-продажи ───────────────────────────────────────────────
	{models.Row{}, "SalesRow"},
	{models.DrilldownRow{}, ""},
	{models.SalesFilterOptions{}, ""},
	{models.SalesDataResponse{}, ""},
	{models.SalesNetworkOptionsResponse{}, ""},
	{models.DrilldownResponse{}, ""},
	{models.SalesDashboardPoint{}, ""},
	{models.SalesDashboardRank{}, ""},
	{models.SalesDashboardSeriesPoint{}, ""},
	{models.SalesDashboardFocusPoint{}, ""},
	{models.SalesDashboardNetworkBreakdown{}, ""},
	{models.SalesDashboardMetricComparison{}, ""},
	{models.SalesDashboardMetricComparisons{}, ""},
	{models.SalesDashboardDriver{}, ""},
	{models.SalesDashboardRankDetail{}, ""},
	{models.SalesDashboardEcomShare{}, ""},
	{models.SalesDashboardSummary{}, ""},
	{models.SalesDashboardResponse{}, ""},

	// ─── Реестр сетей ───────────────────────────────────────────────────
	{models.Network{}, ""},
	{models.NetworkPeriod{}, ""},
	{models.NetworkPlan{}, ""},
	{models.NetworkPlanTotals{}, ""},
	{models.NetworkComment{}, ""},
	{models.AuditLogRow{}, ""},
	{models.NetworkPlanResponse{}, ""},
	{models.NetworkPlanSaveResponse{}, ""},
	{models.NetworkPlanPreviewResponse{}, ""},
	{models.NetworkListResponse{}, ""},
	{models.NetworkSaveResponse{}, ""},
	{models.NetworkCommentsResponse{}, ""},
	{models.NetworkAuditResponse{}, ""},
	{models.NetworkBrandsResponse{}, ""},
	{repository.NetworkPlanInput{}, ""},

	// ─── Промо ──────────────────────────────────────────────────────────
	{models.PromoRow{}, ""},
	{models.HistoryRow{}, ""},
	{models.CommentRow{}, ""},
	{models.ApprovalRow{}, ""},
	{models.NetworkGeo{}, ""},
	{models.LastSKUData{}, ""},
}

const header = `// СГЕНЕРИРОВАНО backend/cmd/tsgen — НЕ РЕДАКТИРОВАТЬ ВРУЧНУЮ.
//
// Источник — Go-структуры в backend/models и backend/repository.
// Пересобрать: make types (из корня проекта).
// CI проверяет, что файл совпадает с исходниками: make types-check.

`

func main() {
	output, err := generate()
	if err != nil {
		log.Fatalf("tsgen: %v", err)
	}

	target := filepath.Join("..", "frontend", "src", "types", "api.generated.ts")
	if len(os.Args) > 1 {
		target = os.Args[1]
	}
	if err := os.WriteFile(target, []byte(output), 0o644); err != nil {
		log.Fatalf("tsgen: запись %s: %v", target, err)
	}
	fmt.Printf("tsgen: %d типов записано в %s\n", len(exported), target)
}

// tsNameOf возвращает имя типа в TypeScript.
func tsNameOf(e entry) string {
	if e.name != "" {
		return e.name
	}
	return reflect.TypeOf(e.value).Name()
}

func generate() (string, error) {
	// Имена известных структур: по ним разрешаются ссылки между типами.
	known := make(map[reflect.Type]string, len(exported))
	for _, e := range exported {
		t := reflect.TypeOf(e.value)
		if t.Kind() != reflect.Struct {
			return "", fmt.Errorf("%s — не структура", t)
		}
		known[t] = tsNameOf(e)
	}

	var b strings.Builder
	b.WriteString(header)

	for i, e := range exported {
		if i > 0 {
			b.WriteString("\n")
		}
		block, err := renderStruct(reflect.TypeOf(e.value), tsNameOf(e), known)
		if err != nil {
			return "", err
		}
		b.WriteString(block)
	}
	return b.String(), nil
}

// renderStruct собирает одно объявление interface.
func renderStruct(t reflect.Type, name string, known map[reflect.Type]string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "export interface %s {\n", name)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" { // неэкспортируемое поле
			continue
		}
		jsonName, omitEmpty, skip := parseJSONTag(field)
		if skip {
			continue
		}

		fieldType := field.Type
		optional := omitEmpty
		nullable := false
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
			// Указатель с omitempty из JSON просто исчезает, null в нём не бывает.
			if !omitEmpty {
				nullable = true
			}
		}

		tsType, err := tsTypeOf(fieldType, known)
		if err != nil {
			return "", fmt.Errorf("%s.%s: %w", name, field.Name, err)
		}
		if nullable {
			tsType += " | null"
		}

		question := ""
		if optional {
			question = "?"
		}
		fmt.Fprintf(&b, "  %s%s: %s;\n", jsonName, question, tsType)
	}

	b.WriteString("}\n")
	return b.String(), nil
}

// parseJSONTag разбирает тег: имя поля, omitempty и признак пропуска.
func parseJSONTag(field reflect.StructField) (name string, omitEmpty, skip bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false, true
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	if name == "" {
		name = field.Name
	}
	for _, option := range parts[1:] {
		if option == "omitempty" {
			omitEmpty = true
		}
	}
	return name, omitEmpty, false
}

// tsTypeOf переводит тип Go в тип TypeScript.
func tsTypeOf(t reflect.Type, known map[reflect.Type]string) (string, error) {
	if name, ok := known[t]; ok {
		return name, nil
	}

	switch t.Kind() {
	case reflect.Bool:
		return "boolean", nil
	case reflect.String:
		return "string", nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number", nil
	case reflect.Interface:
		if t.NumMethod() == 0 {
			return "unknown", nil
		}
		return "", fmt.Errorf("интерфейс %s не переводится в TypeScript", t)
	case reflect.Ptr:
		inner, err := tsTypeOf(t.Elem(), known)
		if err != nil {
			return "", err
		}
		return "(" + inner + " | null)", nil
	case reflect.Slice, reflect.Array:
		inner, err := tsTypeOf(t.Elem(), known)
		if err != nil {
			return "", err
		}
		if t.Elem().Kind() == reflect.Ptr {
			return "Array<" + inner + ">", nil
		}
		return inner + "[]", nil
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return "", fmt.Errorf("ключ карты %s должен быть строкой", t)
		}
		inner, err := tsTypeOf(t.Elem(), known)
		if err != nil {
			return "", err
		}
		return "Record<string, " + inner + ">", nil
	case reflect.Struct:
		return "", fmt.Errorf("структура %s не описана в exported — добавьте её в реестр", t)
	}
	return "", fmt.Errorf("тип %s не поддерживается", t)
}

// init держит реестр без повторов: две записи с одним именем молча
// перетёрли бы друг друга в сгенерированном файле.
func init() {
	names := make(map[string][]string)
	for _, e := range exported {
		names[tsNameOf(e)] = append(names[tsNameOf(e)], reflect.TypeOf(e.value).String())
	}
	var duplicates []string
	for name, sources := range names {
		if len(sources) > 1 {
			duplicates = append(duplicates, fmt.Sprintf("%s ← %s", name, strings.Join(sources, ", ")))
		}
	}
	if len(duplicates) > 0 {
		sort.Strings(duplicates)
		log.Fatalf("tsgen: повторяющиеся имена типов: %s", strings.Join(duplicates, "; "))
	}
}
