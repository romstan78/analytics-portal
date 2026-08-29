package services

import (
	"time"

	"backend/repository"
)

// ─── Зеркало инвестиций в квартальных колонках ──────────────────────────────
//
// Портал считает инвестиции на лету и в этих колонках не нуждается. Нужны они
// внешним потребителям: ежедневной выгрузке, BI и интеграциям, которые читают
// tbl_NetworkPlans напрямую и правило применить не могут — порог смотрит на
// валовый пул и на правила совместного зачёта, одним SELECT это не выражается.
//
// Отсюда и разделение обязанностей: запись обновляет колонки после каждого
// изменения входа правила, а чтение всё равно пересчитывает. Пропущенная запись
// тогда не может показать человеку устаревшие деньги — только задержать их
// снаружи до ближайшего пересчёта.

// RebuildNetworkInvestmentColumns пересчитывает и записывает расчётные колонки
// инвестиций одной сети за год тем же путём, что и вкладка «План и факт»:
// свод помесячного слоя, затем общее правило. Другой путь означал бы вторую
// арифметику, которая однажды разойдётся с экраном.
func RebuildNetworkInvestmentColumns(networkID, year int) error {
	network, err := repository.GetNetworkByID(networkID)
	if err != nil {
		return err
	}
	periods, err := repository.GetNetworkPeriods(networkID, year)
	if err != nil {
		return err
	}
	periods = NetworkPeriodsWithDefaults(network, year, periods)

	plans, err := repository.GetNetworkPlans(networkID, year)
	if err != nil {
		return err
	}
	groups, err := repository.GetNetworkPeriodGroups(networkID, year)
	if err != nil {
		return err
	}
	facts, err := repository.GetNetworkMonthlyFacts(networkID, year-1, year)
	if err != nil {
		return err
	}
	forecasts, err := repository.GetNetworkForecastLines(networkID, year, repository.AllQuarters)
	if err != nil {
		return err
	}
	promos, err := repository.GetNetworkPromoIndicators(network.Name, year, repository.AllQuarters)
	if err != nil {
		return err
	}
	prices, err := repository.GetNetworkContractPrices(networkID, year)
	if err != nil {
		return err
	}

	plans = ApplyForecastRollup(
		network, year, plans, periods, facts, forecasts, promos, prices, groups, time.Now(),
	)
	calculated, _ := BuildNetworkPlanCalculations(plans, periods, groups)
	return repository.SaveNetworkInvestmentColumns(calculated)
}
