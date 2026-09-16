package services

import (
	"fmt"
	"sort"
	"time"

	"backend/models"
	"backend/repository"
)

func budgetExtraLines(b *models.BudgetBrand) map[string]*models.BudgetLine {
	return map[string]*models.BudgetLine{"olap": &b.OlapTo, "ss": &b.SalesSS, "sswo": &b.SalesSSWO, "pure": &b.SalesPURE, "omni": &b.SalesOMNI, "mp": &b.SalesMP}
}
func budgetExtraTotals(b *models.BudgetBrand) {
	for key, line := range budgetExtraLines(b) {
		line.Q = make([]models.BudgetCell, 4)
		line.Year = models.BudgetCell{}
		missing := [4]bool{}
		for i := range b.Networks {
			other := budgetExtraLines(&b.Networks[i])[key]
			for q := 0; q < 4; q++ {
				if key == "olap" && other.Q[q].A == nil && valueOrZero(b.Networks[i].To.Q[q].A) != 0 {
					missing[q] = true
				}
				addPtrValue(&line.Q[q].A, other.Q[q].A)
			}
		}
		incomplete := false
		for q, c := range line.Q {
			if missing[q] {
				line.Q[q].A = nil
				incomplete = true
			} else {
				addPtrValue(&line.Year.A, c.A)
			}
		}
		if incomplete {
			line.Year.A = nil
		}
	}
}

// The undistributed contract pool has no SKU quantities and cannot be valued at
// OLAP prices. It is not a missing price for a real brand and must not invalidate
// the portfolio's OLAP totals. Keep missing values for real brands unchanged.
func budgetPortfolioExtraTotals(r *models.BudgetResponse) {
	r.Total.Networks = []models.BudgetBrand{}
	for _, b := range r.Brands {
		if b.Brand != "Нераспределённый остаток пула" {
			r.Total.Networks = append(r.Total.Networks, b.Networks...)
		}
	}
	budgetExtraTotals(&r.Total)
	r.Total.Networks = []models.BudgetBrand{}
}

func budgetPresentation(r *models.BudgetResponse, f BudgetFilter) {
	apply := func(b *models.BudgetBrand) {
		registryBase := &b.To
		if f.Base == "reg-olap" {
			registryBase = &b.OlapTo
		}
		for q := 0; q < 4; q++ {
			if salesLine := budgetOLAPSourceLine(f.ToSources[q]); salesLine != "" {
				b.To.Q[q] = budgetExtraLines(b)[salesLine].Q[q]
			} else {
				b.To.Q[q] = registryBase.Q[q]
			}
			b.Pct.Q[q].A = nil
			if b.To.Q[q].A != nil && *b.To.Q[q].A != 0 {
				b.Pct.Q[q].A = models.PtrFloat(round2(valueOrZero(b.Investments.Q[q].A) / *b.To.Q[q].A * 100))
			}
		}
		b.To.Year.A = nil
		incomplete := false
		for q := 0; q < 4; q++ {
			if b.To.Q[q].A == nil {
				incomplete = true
				continue
			}
			addPtrValue(&b.To.Year.A, b.To.Q[q].A)
		}
		if incomplete {
			b.To.Year.A = nil
		}
		b.Pct.Year.A = nil
		if b.To.Year.A != nil && *b.To.Year.A != 0 {
			b.Pct.Year.A = models.PtrFloat(round2(valueOrZero(b.Investments.Year.A) / *b.To.Year.A * 100))
		}
	}
	for i := range r.Brands {
		for j := range r.Brands[i].Networks {
			apply(&r.Brands[i].Networks[j])
		}
		apply(&r.Brands[i])
	}
	apply(&r.Total)
}
func addBudgetBases(r *models.BudgetResponse, data repository.NetworkDashboardData, f BudgetFilter, prices []repository.BudgetPrice, sales []repository.BudgetSale, now time.Time) {
	current, prev := indexPeriodData(data.Current), indexPeriodData(data.Prev)
	uplifts := indexPromos(data.Promos, func(_ string, _ int, v float64) float64 { return v }).forecastUplifts
	pp := []models.NetworkContractPrice{}
	skusByBrand := map[string][]string{}
	priceBySKU := map[string]float64{}
	for _, p := range prices {
		pp = append(pp, models.NetworkContractPrice{SKU: p.SKU, BrandAS: p.Brand, ContractPrice: p.Price, ValidFrom: "1900-01-01", ValidTo: "9999-12-31"})
		skusByBrand[p.Brand] = append(skusByBrand[p.Brand], p.SKU)
		priceBySKU[p.SKU] = p.Price
	}
	for b := range skusByBrand {
		sort.Strings(skusByBrand[b])
	}
	netIDs := map[int]models.Network{}
	for _, n := range data.Networks {
		netIDs[n.ID] = n
	}
	// Include sales-only brands in the same network scope before creating pointers.
	for _, sale := range sales {
		if _, ok := netIDs[sale.NetworkID]; !ok {
			continue
		}
		bi := -1
		for i := range r.Brands {
			if r.Brands[i].Brand == sale.Brand {
				bi = i
				break
			}
		}
		if bi < 0 {
			r.Brands = append(r.Brands, budgetBrand(sale.Brand, 0, [4]budgetAmounts{}, f))
			bi = len(r.Brands) - 1
		}
		found := false
		for _, n := range r.Brands[bi].Networks {
			if n.NetworkID == sale.NetworkID {
				found = true
			}
		}
		if !found {
			r.Brands[bi].Networks = append(r.Brands[bi].Networks, budgetBrand(netIDs[sale.NetworkID].Name, sale.NetworkID, [4]budgetAmounts{}, f))
		}
	}
	sort.Slice(r.Brands, func(i, j int) bool { return r.Brands[i].Brand < r.Brands[j].Brand })
	for i := range r.Brands {
		sort.Slice(r.Brands[i].Networks, func(a, b int) bool { return r.Brands[i].Networks[a].NetworkID < r.Brands[i].Networks[b].NetworkID })
	}
	rows := map[string]*models.BudgetBrand{}
	key := func(id int, brand string) string { return fmt.Sprintf("%d|%s", id, brand) }
	for bi := range r.Brands {
		b := &r.Brands[bi]
		for ni := range b.Networks {
			n := &b.Networks[ni]
			rows[key(n.NetworkID, b.Brand)] = n
			facts := append(append([]models.NetworkMonthlyFact{}, prev.facts[n.NetworkID]...), current.facts[n.NetworkID]...)
			brandFacts, skuFacts := aggregateFacts(facts)
			official := aggregateForecastLines(current.forecasts[n.NetworkID])
			distribution := networkMonthlyDistribution(netIDs[n.NetworkID])
			for q := 0; q < 4; q++ {
				var plan *models.NetworkPlan
				for i := range current.plans[n.NetworkID] {
					p := &current.plans[n.NetworkID][i]
					if p.BrandAS != nil && *p.BrandAS == b.Brand && p.Quarter == q+1 {
						plan = p
						break
					}
				}
				if plan == nil {
					continue
				}
				total := 0.0
				covered := true
				for i := 0; i < 3; i++ {
					month := q*3 + i + 1
					k := forecastMonthKey(f.Year, month, b.Brand)
					units := brandFacts[k].units
					source := f.ToSources[q]
					if source == "plan" {
						if plan.PlanUnits != nil {
							units = models.PtrFloat(*plan.PlanUnits * distribution[i] / 100)
						}
					} else if source != "fact" {
						sys, _ := recommendedForecastMetric(brandFacts, f.Year, month, b.Brand, uplifts[promoForecastKey{network: netIDs[n.NetworkID].Name, brand: b.Brand, month: month}].units, unitsMetric)
						units = valueForEAC(isClosedForecastMonth(f.Year, month, now), units, official[k].units, sys)
					}
					if units == nil || *units == 0 {
						continue
					}
					// Detailed facts/forecasts use their SKU values; brand-level input uses the
					// same historical SKU mix and weighted-price helper as the registry.
					detailed := false
					detailValue := 0.0
					detailUnits := 0.0
					if source == "fact" || (source != "plan" && isClosedForecastMonth(f.Year, month, now)) {
						for _, fact := range current.facts[n.NetworkID] {
							if fact.BrandAS == b.Brand && fact.Month == month && fact.SKU != nil && fact.FactUnits != nil {
								price, ok := priceBySKU[*fact.SKU]
								if !ok && *fact.FactUnits != 0 {
									covered = false
								}
								detailValue += *fact.FactUnits * price
								detailUnits += *fact.FactUnits
								detailed = true
							}
						}
					} else if source != "plan" && plan.EntryLevel == "sku" {
						for _, line := range current.forecasts[n.NetworkID] {
							if line.BrandAS == b.Brand && line.Month == month && line.SKU != nil && line.ForecastUnits != nil {
								price, ok := priceBySKU[*line.SKU]
								if !ok && *line.ForecastUnits != 0 {
									covered = false
								}
								detailValue += *line.ForecastUnits * price
								detailUnits += *line.ForecastUnits
								detailed = true
							}
						}
					}
					if detailed && round2(detailUnits) == round2(*units) {
						total += detailValue
						continue
					}
					skus := skusByBrand[b.Brand]
					shares := skuMixShares(skuFacts, f.Year, month, b.Brand, skus)
					price := brandWeightedPrice(pp, b.Brand, f.Year, month, skus, shares)
					if price == nil {
						covered = false
					} else {
						total += *units * *price
					}
				}
				if covered {
					n.OlapTo.Q[q].A = models.PtrFloat(round2(total))
				}
				addPtrValue(&n.OlapTo.Year.A, n.OlapTo.Q[q].A)
			}
		}
	}
	for _, s := range sales {
		if !isClosedForecastMonth(f.Year, s.Month, now) {
			continue
		}
		n := rows[key(s.NetworkID, s.Brand)]
		if n == nil {
			continue
		}
		q := (s.Month - 1) / 3
		keys := []string{}
		if s.Segment == "OLAP SS" {
			keys = append(keys, "ss")
		}
		if s.Segment == "OLAP SS wo Ecom" {
			keys = append(keys, "sswo")
		}
		switch s.Channel {
		case "PURE":
			keys = append(keys, "pure")
		case "OMNI":
			keys = append(keys, "omni")
		case "MP":
			keys = append(keys, "mp")
		}
		for _, k := range keys {
			line := budgetExtraLines(n)[k]
			addPtrValue(&line.Q[q].A, models.PtrFloat(s.Rub))
			addPtrValue(&line.Year.A, models.PtrFloat(s.Rub))
		}
	}
	for i := range r.Brands {
		budgetExtraTotals(&r.Brands[i])
	}
	budgetPortfolioExtraTotals(r)
}
