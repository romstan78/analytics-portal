package services

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/encoding/charmap"
)

// Официальные курсы ЦБ РФ. Дашборд показывает суммы в евро по среднему курсу
// за месяц, поэтому котировки берутся один раз на год и кешируются.

const cbrEURCurrencyID = "R01239"

type cbrCurrencyRecord struct {
	Date    string `xml:"Date,attr"`
	Nominal string `xml:"Nominal"`
	Value   string `xml:"Value"`
}

type cbrCurrencyResponse struct {
	Records []cbrCurrencyRecord `xml:"Record"`
}

type eurRateCacheEntry struct {
	Rates     map[int]float64
	ExpiresAt time.Time
}

var eurRateCache = struct {
	sync.Mutex
	Items map[int]eurRateCacheEntry
}{Items: make(map[int]eurRateCacheEntry)}

func parseCBRDecimal(value string) (float64, error) {
	return strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(value), ",", "."), 64)
}

// parseEURMonthlyRates усредняет дневные котировки внутри каждого месяца.
func parseEURMonthlyRates(reader io.Reader) (map[int]float64, error) {
	decoder := xml.NewDecoder(reader)
	decoder.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) {
		return charmap.Windows1251.NewDecoder().Reader(input), nil
	}
	var response cbrCurrencyResponse
	if err := decoder.Decode(&response); err != nil {
		return nil, err
	}
	sums := make(map[int]float64)
	counts := make(map[int]int)
	for _, record := range response.Records {
		date, err := time.Parse("02.01.2006", record.Date)
		if err != nil {
			continue
		}
		nominal, err := parseCBRDecimal(record.Nominal)
		if err != nil || nominal == 0 {
			continue
		}
		value, err := parseCBRDecimal(record.Value)
		if err != nil {
			continue
		}
		sums[int(date.Month())] += value / nominal
		counts[int(date.Month())]++
	}
	rates := make(map[int]float64, len(sums))
	for month, sum := range sums {
		if counts[month] > 0 {
			rates[month] = sum / float64(counts[month])
		}
	}
	if len(rates) == 0 {
		return nil, fmt.Errorf("ЦБ РФ не вернул курсы EUR")
	}
	return rates, nil
}

// fillMissingMonths достраивает месяцы, за которые ЦБ ещё не публиковал курс.
//
// Дневных котировок нет за незакрытый остаток текущего года, а суммы за такие
// месяцы в витрине встречаются: планы и демонстрационный контур заполнены
// вперёд. Без курса пересчёт в евро возвращал бы ошибку на весь дашборд, хотя
// не хватает котировки за один месяц. Пропуск закрывается последним известным
// курсом; когда ЦБ опубликует настоящий, он заменит перенесённый при
// ближайшем обновлении кеша.
func fillMissingMonths(rates map[int]float64) map[int]float64 {
	filled := make(map[int]float64, 12)
	carried := 0.0
	for month := 1; month <= 12; month++ {
		if rate, ok := rates[month]; ok && rate > 0 {
			carried = rate
		}
		if carried > 0 {
			filled[month] = carried
		}
	}
	// Месяцы до первой котировки закрываются первым известным курсом: год
	// может начинаться с пропуска, если выборка ЦБ стартовала не с января.
	first := 0.0
	for month := 1; month <= 12; month++ {
		if filled[month] > 0 {
			first = filled[month]
			break
		}
	}
	if first > 0 {
		for month := 1; month <= 12; month++ {
			if filled[month] <= 0 {
				filled[month] = first
			}
		}
	}
	return filled
}

// LoadEURMonthlyRates возвращает средние месячные курсы EUR за год.
func LoadEURMonthlyRates(year int) (map[int]float64, error) {
	eurRateCache.Lock()
	if cached, ok := eurRateCache.Items[year]; ok && time.Now().Before(cached.ExpiresAt) {
		eurRateCache.Unlock()
		return cached.Rates, nil
	}
	eurRateCache.Unlock()

	start := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(year, time.December, 31, 0, 0, 0, 0, time.UTC)
	now := time.Now()
	if year == now.Year() && now.Before(end) {
		end = now
	}
	if year > now.Year() {
		return nil, fmt.Errorf("курсы EUR за %d год ещё недоступны", year)
	}
	params := url.Values{
		"date_req1": {start.Format("02/01/2006")},
		"date_req2": {end.Format("02/01/2006")},
		"VAL_NM_RQ": {cbrEURCurrencyID},
	}
	request, err := http.NewRequest(http.MethodGet, "https://www.cbr.ru/scripts/XML_dynamic.asp?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "AnalyticsPortal/1.0")
	client := &http.Client{Timeout: 12 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ЦБ РФ вернул HTTP %d", response.StatusCode)
	}
	rates, err := parseEURMonthlyRates(response.Body)
	if err != nil {
		return nil, err
	}
	rates = fillMissingMonths(rates)
	eurRateCache.Lock()
	eurRateCache.Items[year] = eurRateCacheEntry{Rates: rates, ExpiresAt: time.Now().Add(6 * time.Hour)}
	eurRateCache.Unlock()
	return rates, nil
}
