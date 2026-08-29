package services

import (
	"time"

	"backend/models"
	"backend/repository"
)

// ─── Пара «рубли / упаковки» в строках прогноза ─────────────────────────────
//
// Прогноз вводится в одной единице: у бренда есть режим ведения, и вторая
// величина считается из введённой по цене контракта того же месяца. Пересчёт
// живёт в BuildNetworkForecast — форма всегда показывала обе.
//
// В БД, однако, попадала только введённая. Форме этого хватало, потому что она
// пересчитывает при каждом чтении, а вот всем, кто читает колонки напрямую, —
// нет: витрина реестра берёт forecast_rub и forecast_units сырыми и цен не
// загружает вовсе, поэтому прогноз, внесённый в рублях, в разрезе упаковок для
// неё просто не существовал. То же у ночных выгрузок и BI.
//
// Отсюда разделение обязанностей, такое же, как у зеркала инвестиций: чтение
// по-прежнему считает пару само, а запись закрепляет посчитанное в БД после
// каждого изменения входа — самого прогноза, режима ведения бренда или цены.
// Пропущенная запись тогда не показывает человеку устаревшие числа, а лишь
// задерживает их снаружи до ближайшего пересчёта.

// LoadNetworkForecast собирает рабочее место прогноза одной сети за квартал.
//
// Живёт в services, а не в обработчике: тем же путём ходит пакетный пересчёт
// пары, у которого HTTP-запроса нет вовсе, а второй путь означал бы вторую
// арифметику, которая однажды разойдётся с экраном.
func LoadNetworkForecast(networkID, year, quarter int) (models.NetworkForecastResponse, error) {
	network, err := repository.GetNetworkByID(networkID)
	if err != nil {
		return models.NetworkForecastResponse{}, err
	}
	periods, err := repository.GetNetworkPeriods(networkID, year)
	if err != nil {
		return models.NetworkForecastResponse{}, err
	}
	periods = NetworkPeriodsWithDefaults(network, year, periods)
	plans, err := repository.GetNetworkPlans(networkID, year)
	if err != nil {
		return models.NetworkForecastResponse{}, err
	}
	facts, err := repository.GetNetworkMonthlyFacts(networkID, year-1, year)
	if err != nil {
		return models.NetworkForecastResponse{}, err
	}
	forecasts, err := repository.GetNetworkForecastLines(networkID, year, quarter)
	if err != nil {
		return models.NetworkForecastResponse{}, err
	}
	promos, err := repository.GetNetworkPromoIndicators(network.Name, year, quarter)
	if err != nil {
		return models.NetworkForecastResponse{}, err
	}
	prices, err := repository.GetNetworkContractPrices(networkID, year)
	if err != nil {
		return models.NetworkForecastResponse{}, err
	}
	groups, err := repository.GetNetworkPeriodGroups(networkID, year)
	if err != nil {
		return models.NetworkForecastResponse{}, err
	}
	return BuildNetworkForecast(
		network, year, quarter, plans, periods, facts, forecasts, promos, prices, groups, time.Now(),
	), nil
}

// RebuildForecastPairs пересчитывает квартал и записывает обе метрики пары.
func RebuildForecastPairs(networkID, year, quarter int) (int64, error) {
	response, err := LoadNetworkForecast(networkID, year, quarter)
	if err != nil {
		return 0, err
	}
	return repository.SyncNetworkForecastPairs(networkID, year, quarter, response.Months)
}

// RebuildForecastPairsYear — то же за все четыре квартала. Нужен после правки
// цен: прайс заведён периодами, и одна строка цены может задеть любой из них.
func RebuildForecastPairsYear(networkID, year int) (int64, error) {
	var written int64
	for quarter := 1; quarter <= 4; quarter++ {
		affected, err := RebuildForecastPairs(networkID, year, quarter)
		if err != nil {
			return written, err
		}
		written += affected
	}
	return written, nil
}
