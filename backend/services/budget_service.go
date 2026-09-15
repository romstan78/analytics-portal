package services

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"backend/models"
	"backend/repository"
)

type BudgetFilter struct {
	Composition    bool
	ExpandedBrands []string
	Year           int
	Source, Kind   string
	Base           string
	PastYear       bool
	Gross          bool
	NetworkTypes   []string
	// Sources are selected by the user independently for each quarter.
	ToSources         [4]string
	InvestmentSources [4]string
}

// budgetAmounts retains the independent measures used for reconciliation.
type budgetAmounts struct {
	to, plan, fact, gtnPlan, gtn, opex, promoGTN, promoOPEX float64
	mark                                                    string
}

func (a *budgetAmounts) add(b budgetAmounts) {
	a.to = round2(a.to + b.to)
	a.plan = round2(a.plan + b.plan)
	a.fact = round2(a.fact + b.fact)
	a.gtnPlan = round2(a.gtnPlan + b.gtnPlan)
	a.gtn = round2(a.gtn + b.gtn)
	a.opex = round2(a.opex + b.opex)
	a.promoGTN = round2(a.promoGTN + b.promoGTN)
	a.promoOPEX = round2(a.promoOPEX + b.promoOPEX)
	if a.mark == "" {
		a.mark = b.mark
	} else if b.mark != "" && a.mark != b.mark {
		a.mark = "partial"
	}
}

func budgetValue(gross, net float64, f BudgetFilter) float64 {
	if f.Gross {
		return gross
	}
	return net
}

func budgetBrand(name string, id int, values [4]budgetAmounts, f BudgetFilter) models.BudgetBrand {
	r := models.BudgetBrand{Brand: name, NetworkID: id, Networks: []models.BudgetBrand{}}
	for _, line := range budgetExtraLines(&r) {
		line.Q = make([]models.BudgetCell, 4)
	}
	r.Sales.Q = make([]models.BudgetCell, 4)
	lines := []*models.BudgetLine{&r.To, &r.Plan, &r.Fact, &r.GtnPlan, &r.GtnContract, &r.OpexContract, &r.GtnPromo, &r.OpexPromo, &r.Investments, &r.Pct}
	var annual budgetAmounts
	for _, v := range values {
		annual.add(v)
	}
	for i, v := range append(values[:], annual) {
		invest := 0.0
		if f.Source != "promo" {
			if f.Kind != "opex" {
				invest += v.gtn
			}
			if f.Kind != "gtn" {
				invest += v.opex
			}
		}
		if f.Source != "contract" {
			if f.Kind != "opex" {
				invest += v.promoGTN
			}
			if f.Kind != "gtn" {
				invest += v.promoOPEX
			}
		}
		numbers := []float64{v.to, v.plan, v.fact, v.gtnPlan, v.gtn, v.opex, v.promoGTN, v.promoOPEX, round2(invest), 0}
		for j, line := range lines {
			cell := models.BudgetCell{A: models.PtrFloat(numbers[j]), Mark: "none"}
			if j == 4 || (j == 8 && f.Source != "promo" && f.Kind != "opex") {
				cell.Mark = v.mark
			}
			if j == 9 {
				cell.A = nil
				if v.to != 0 {
					cell.A = models.PtrFloat(round2(invest / v.to * 100))
				}
			}
			if i == 4 {
				line.Year = cell
			} else {
				line.Q = append(line.Q, cell)
			}
		}
	}
	return r
}

// AggregateBudgetRegistry reuses the registry's monthly and threshold calculation.
// The result deliberately exposes plan/fact and planned GTN for reconciliation.
func AggregateBudgetRegistry(data repository.NetworkDashboardData, f BudgetFilter, now time.Time) *models.BudgetResponse {
	r := &models.BudgetResponse{Year: f.Year, Version: "LIVE", Brands: []models.BudgetBrand{}, NetworkTypes: []string{}, QuarterStates: []string{}}
	for q := 1; q <= 4; q++ {
		state := "future"
		if isClosedForecastMonth(f.Year, q*3, now) {
			state = "closed"
		} else if isClosedForecastMonth(f.Year, q*3-2, now) || (now.Year() == f.Year && (int(now.Month())-1)/3+1 == q) {
			state = "current"
		}
		r.QuarterStates = append(r.QuarterStates, state)
	}
	current, previous := indexPeriodData(data.Current), indexPeriodData(data.Prev)
	net := promoNetRub(data.Networks, current.periods, f.Year)
	promos := indexPromos(data.Promos, net)
	byBrand := map[string]map[int]*[4]budgetAmounts{}
	get := func(brand string, id int) *[4]budgetAmounts {
		if byBrand[brand] == nil {
			byBrand[brand] = map[int]*[4]budgetAmounts{}
		}
		if byBrand[brand][id] == nil {
			byBrand[brand][id] = &[4]budgetAmounts{}
		}
		return byBrand[brand][id]
	}
	names := map[int]string{}
	ids := map[string]int{}
	for _, network := range data.Networks {
		names[network.ID] = network.Name
		ids[network.Name] = network.ID
		facts := append(append([]models.NetworkMonthlyFact{}, previous.facts[network.ID]...), current.facts[network.ID]...)
		slice := buildNetworkSlice(network, f.Year, map[int]bool{1: true, 2: true, 3: true, 4: true}, current.plans[network.ID], current.periods[network.ID], current.facts[network.ID], facts, current.forecasts[network.ID], promos.forecastUplifts, current.groups[network.ID], current.opex[network.ID], now)
		for key, v := range slice.brandQuarterValues {
			row := get(key.brand, network.ID)
			gtn, mark := budgetValue(v.eacInvest, v.eacInvestNet, f), "forecast"
			if f.InvestmentSources[key.quarter-1] == "plan" {
				gtn = budgetValue(v.planInvest, v.planInvestNet, f)
				mark = "budget"
			}
			if f.InvestmentSources[key.quarter-1] == "fact" {
				gtn = 0
				mark = "paid"
			}
			to := v.eacRub
			if f.ToSources[key.quarter-1] == "plan" {
				to = v.planRub
			}
			if f.ToSources[key.quarter-1] == "fact" {
				to = v.factRub
			}
			row[key.quarter-1].add(budgetAmounts{to: to, plan: v.planRub, fact: v.factRub, gtnPlan: budgetValue(v.planInvest, v.planInvestNet, f), gtn: gtn, opex: budgetValue(v.opex.rub, v.opex.net, f), mark: mark})
		}
		// Payment is an alternative to the forecast, not an extra investment.
		for _, p := range current.plans[network.ID] {
			if p.BrandAS == nil || p.Quarter < 1 || p.Quarter > 4 || p.PaidInvestmentsRub == nil || f.InvestmentSources[p.Quarter-1] != "fact" {
				continue
			}
			row := get(strings.TrimSpace(*p.BrandAS), network.ID)
			row[p.Quarter-1].gtn = budgetValue(*p.PaidInvestmentsRub, net(network.Name, p.Quarter, *p.PaidInvestmentsRub), f)
			row[p.Quarter-1].mark = "paid"
		}
		annualPlans := append([]models.NetworkPlan{}, slice.calculatedPlans...)
		for i := range annualPlans {
			for _, saved := range current.plans[network.ID] {
				if saved.Quarter == annualPlans[i].Quarter && valueOrEmpty(saved.BrandAS) == valueOrEmpty(annualPlans[i].BrandAS) {
					annualPlans[i].PaidInvestmentsRub = saved.PaidInvestmentsRub
					break
				}
			}
		}
		annualTotals := []models.NetworkPlanTotals{}
		for _, t := range slice.quarterTotals {
			annualTotals = append(annualTotals, t)
		}
		annualBudget := CalculateNetworkAnnualInvestmentCumulativeForNetwork(network, annualPlans, NetworkPeriodsWithDefaults(network, f.Year, current.periods[network.ID]), annualTotals)
		if annual := annualBudget; annual != nil && f.InvestmentSources[3] != "plan" && f.InvestmentSources[3] != "fact" {
			for _, supplement := range annual.Rows {
				brand := strings.TrimSpace(valueOrEmpty(supplement.BrandAS))
				if brand == "" {
					brand = "Нераспределённый остаток пула"
				}
				row := get(brand, network.ID)
				row[3].gtn = round2(row[3].gtn + budgetValue(supplement.SupplementRub, supplement.SupplementRubNet, f))
			}
		}
		for q, t := range slice.quarterTotals {
			if t.Undistributed != nil && *t.Undistributed != 0 {
				get("Нераспределённый остаток пула", network.ID)[q-1].plan = round2(*t.Undistributed)
				if f.ToSources[q-1] == "plan" {
					get("Нераспределённый остаток пула", network.ID)[q-1].to = round2(*t.Undistributed)
				}
			}
		}
	}
	for _, p := range data.Promos {
		id, ok := ids[p.NetworkName]
		if !ok || p.Month < 1 || p.Month > 12 {
			continue
		}
		brand := strings.TrimSpace(valueOrEmpty(p.BrandAS))
		if brand == "" {
			brand = "Без бренда"
		}
		q := (p.Month - 1) / 3
		value := budgetValue(p.EffectiveInvest, net(p.NetworkName, q+1, p.EffectiveInvest), f)
		row := get(brand, id)
		if isOPEXInvestment(p.GTNOpex) {
			row[q].promoOPEX = round2(row[q].promoOPEX + value)
		} else {
			row[q].promoGTN = round2(row[q].promoGTN + value)
		}
	}
	var total [4]budgetAmounts
	brands := make([]string, 0, len(byBrand))
	for b := range byBrand {
		brands = append(brands, b)
	}
	sort.Strings(brands)
	for _, b := range brands {
		var values [4]budgetAmounts
		networkIDs := make([]int, 0, len(byBrand[b]))
		for id := range byBrand[b] {
			networkIDs = append(networkIDs, id)
		}
		sort.Ints(networkIDs)
		networks := []models.BudgetBrand{}
		for _, id := range networkIDs {
			v := byBrand[b][id]
			for q := 0; q < 4; q++ {
				values[q].add(v[q])
				total[q].add(v[q])
			}
			networks = append(networks, budgetBrand(names[id], id, *v, f))
		}
		row := budgetBrand(b, 0, values, f)
		row.Networks = networks
		r.Brands = append(r.Brands, row)
	}
	open, covered := 0, 0
	for _, p := range data.Current.Plans {
		if p.BrandAS == nil || isClosedForecastMonth(f.Year, p.Quarter*3, now) {
			continue
		}
		open++
		complete := true
		official := aggregateForecastLines(current.forecasts[p.NetworkID])
		for m := p.Quarter*3 - 2; m <= p.Quarter*3; m++ {
			if !isClosedForecastMonth(f.Year, m, now) && official[forecastMonthKey(f.Year, m, *p.BrandAS)].rub == nil {
				complete = false
			}
		}
		if complete {
			covered++
		}
	}
	if open > 0 {
		r.ForecastCoveragePct = models.PtrFloat(round2(float64(covered) / float64(open) * 100))
	}
	r.Total = budgetBrand("Итого", 0, total, f)
	return r
}

func loadBudgetDraft(f BudgetFilter, overrides []models.BudgetPromo) (*models.BudgetResponse, []models.BudgetPromo, error) {
	types, err := repository.BudgetNetworkTypes()
	if err != nil {
		return nil, nil, err
	}
	filter := repository.NetworkDashboardFilter{Year: f.Year}
	available := map[string]bool{}
	for id, kinds := range types {
		selected := false
		for _, kind := range kinds {
			available[kind] = true
			for _, want := range f.NetworkTypes {
				if kind == want {
					selected = true
				}
			}
		}
		if selected {
			filter.NetworkIDs = append(filter.NetworkIDs, id)
		}
	}
	// An empty selection in the dashboard means all networks; use an impossible
	// ID when a requested type has no networks, so it cannot widen the scope.
	if len(f.NetworkTypes) > 0 && len(filter.NetworkIDs) == 0 {
		filter.NetworkIDs = []int{-1}
	}
	data, err := repository.GetNetworkDashboardData(filter)
	if err != nil {
		return nil, nil, err
	}
	promoRows, err := repository.BudgetPromos(f.Year)
	if err != nil {
		return nil, nil, err
	}
	idx := indexPeriodData(data.Current)
	net := promoNetRub(data.Networks, idx.periods, f.Year)
	allowed := map[int]bool{}
	for _, n := range data.Networks {
		allowed[n.ID] = true
	}
	inclusion := map[int]bool{}
	for _, p := range overrides {
		inclusion[p.ID] = p.Included
	}
	for i := range data.Promos {
		data.Promos[i].EffectiveInvest = 0
		data.Promos[i].FactInvest = 0
		data.Promos[i].InvestRub = 0
	}
	filtered := []models.BudgetPromo{}
	for _, p := range promoRows {
		if !allowed[p.NetworkID] {
			continue
		}
		p.Included = p.Status != "черновик" && p.Status != "исключено"
		if inc, ok := inclusion[p.ID]; ok {
			p.Included = inc
		}
		if p.Status == "исключено" {
			p.Included = false
		}
		p.BudgetRub = p.PlanRub
		if p.FactRub > 0 {
			p.BudgetRub = p.FactRub
		}
		if f.PastYear {
			p.BudgetRub = p.FactRub
		}
		p.BudgetNet = net(p.Network, p.Quarter, p.BudgetRub)
		if p.Included {
			data.Promos = append(data.Promos, repository.NetworkDashboardPromoRow{NetworkName: p.Network, Month: p.Month, BrandAS: models.PtrString(p.Brand), GTNOpex: models.PtrString(p.Type), EffectiveInvest: p.BudgetRub})
		}
		filtered = append(filtered, p)
	}
	promoRows = filtered
	if f.PastYear {
		seen := map[string]bool{}
		for _, p := range data.Current.Plans {
			if p.BrandAS != nil {
				seen[fmt.Sprintf("%d|%s|%d", p.NetworkID, *p.BrandAS, p.Quarter)] = true
			}
		}
		for _, fact := range data.Current.Facts {
			q := (fact.Month-1)/3 + 1
			k := fmt.Sprintf("%d|%s|%d", fact.NetworkID, fact.BrandAS, q)
			if !seen[k] {
				data.Current.Plans = append(data.Current.Plans, models.NetworkPlan{NetworkID: fact.NetworkID, Year: f.Year, Quarter: q, BrandAS: models.PtrString(fact.BrandAS)})
				seen[k] = true
			}
		}
	}
	r := AggregateBudgetRegistry(data, f, time.Now())
	vatFactors := map[int][]float64{}
	for _, n := range data.Networks {
		v := []float64{1, 1, 1, 1}
		for _, p := range NetworkPeriodsWithDefaults(n, f.Year, idx.periods[n.ID]) {
			if p.VATIncluded {
				v[p.Quarter-1] = 1 + p.VATRate/100
			}
		}
		vatFactors[n.ID] = v
	}
	for i := range r.Brands {
		for j := range r.Brands[i].Networks {
			n := &r.Brands[i].Networks[j]
			n.VATFactors = vatFactors[n.NetworkID]
		}
	}
	prices, err := repository.BudgetPricesAt(time.Now())
	if err != nil {
		return nil, nil, err
	}
	sales, err := repository.BudgetSales(f.Year)
	if err != nil {
		return nil, nil, err
	}
	addBudgetBases(r, data, f, prices, sales, time.Now())
	r.TypeMembers = map[string][]int{}
	for id, kinds := range types {
		for _, kind := range kinds {
			r.TypeMembers[kind] = append(r.TypeMembers[kind], id)
		}
	}
	for kind := range available {
		r.NetworkTypes = append(r.NetworkTypes, kind)
	}
	sort.Strings(r.NetworkTypes)
	return r, promoRows, nil
}
