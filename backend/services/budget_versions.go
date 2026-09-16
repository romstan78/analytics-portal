package services

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"backend/models"
	"backend/repository"
)

func budgetLines(b *models.BudgetBrand) map[string]*models.BudgetLine {
	return map[string]*models.BudgetLine{"to": &b.To, "plan": &b.Plan, "fact": &b.Fact, "gtnPlan": &b.GtnPlan, "gtnC": &b.GtnContract, "opexC": &b.OpexContract, "gtnP": &b.GtnPromo, "opexP": &b.OpexPromo}
}
func budgetAmountsOf(b *models.BudgetBrand, q int) budgetAmounts {
	return budgetAmounts{to: valueOrZero(b.To.Q[q].A), plan: valueOrZero(b.Plan.Q[q].A), fact: valueOrZero(b.Fact.Q[q].A), gtnPlan: valueOrZero(b.GtnPlan.Q[q].A), gtn: valueOrZero(b.GtnContract.Q[q].A), opex: valueOrZero(b.OpexContract.Q[q].A), promoGTN: valueOrZero(b.GtnPromo.Q[q].A), promoOPEX: valueOrZero(b.OpexPromo.Q[q].A), mark: b.GtnContract.Q[q].Mark}
}
func budgetRebuild(r *models.BudgetResponse, f BudgetFilter) {
	var total [4]budgetAmounts
	for bi := range r.Brands {
		b := &r.Brands[bi]
		var amounts [4]budgetAmounts
		for ni := range b.Networks {
			n := &b.Networks[ni]
			var own [4]budgetAmounts
			for q := 0; q < 4; q++ {
				own[q] = budgetAmountsOf(n, q)
				amounts[q].add(own[q])
				total[q].add(own[q])
			}
			fresh := budgetBrand(n.Brand, n.NetworkID, own, f)
			fresh.VATFactors = n.VATFactors
			for k, l := range budgetExtraLines(n) {
				*budgetExtraLines(&fresh)[k] = *l
			}
			for metric, line := range budgetLines(n) {
				dest := budgetLines(&fresh)[metric]
				for q := 0; q < 4; q++ {
					dest.Q[q].Edited = line.Q[q].Edited
				}
			}
			*n = fresh
		}
		fresh := budgetBrand(b.Brand, 0, amounts, f)
		fresh.Networks = b.Networks
		for metric, line := range budgetLines(&fresh) {
			for q := 0; q < 4; q++ {
				var edit *models.BudgetEdit
				for ni := range fresh.Networks {
					cell := budgetLines(&fresh.Networks[ni])[metric].Q[q]
					if cell.Edited != nil {
						if edit == nil {
							edit = &models.BudgetEdit{Who: cell.Edited.Who, When: cell.Edited.When}
						}
						edit.Base = round2(edit.Base + cell.Edited.Base)
						edit.Value = round2(edit.Value + cell.Edited.Value)
					}
				}
				line.Q[q].Edited = edit
			}
		}
		*b = fresh
		budgetExtraTotals(b)
	}
	r.Total = budgetBrand("Итого", 0, total, f)
	budgetPortfolioExtraTotals(r)
}
func budgetApplyLines(r *models.BudgetResponse, lines []repository.BudgetStoredLine, f BudgetFilter) {
	for _, l := range lines {
		if !budgetTypeAllowed(r, f, l.NetworkID) {
			continue
		}
		bi := -1
		for i := range r.Brands {
			if r.Brands[i].Brand == l.Brand {
				bi = i
				break
			}
		}
		if bi < 0 {
			r.Brands = append(r.Brands, budgetBrand(l.Brand, 0, [4]budgetAmounts{}, f))
			bi = len(r.Brands) - 1
		}
		b := &r.Brands[bi]
		ni := -1
		for i := range b.Networks {
			if b.Networks[i].NetworkID == l.NetworkID {
				ni = i
				break
			}
		}
		if ni < 0 {
			b.Networks = append(b.Networks, budgetBrand(l.NetworkName, l.NetworkID, [4]budgetAmounts{}, f))
			ni = len(b.Networks) - 1
		}
		line := budgetLines(&b.Networks[ni])[l.Metric]
		if line == nil {
			line = budgetExtraLines(&b.Networks[ni])[l.Metric]
		}
		if line == nil || l.Quarter < 1 || l.Quarter > 4 {
			continue
		}
		cell := &line.Q[l.Quarter-1]
		cell.A = l.Net
		if f.Gross {
			cell.A = l.Gross
		}
		cell.Mark = l.Mark
		if l.Source == "manual" {
			base := l.BaseNet
			if f.Gross {
				base = l.BaseGross
			}
			cell.Edited = &models.BudgetEdit{Base: valueOrZero(base), Value: valueOrZero(cell.A), Who: l.Who, When: l.When}
		}
	}
	sort.Slice(r.Brands, func(i, j int) bool { return r.Brands[i].Brand < r.Brands[j].Brand })
	budgetRebuild(r, f)
}
func budgetSnapshot(net, gross *models.BudgetResponse) []repository.BudgetStoredLine {
	out := []repository.BudgetStoredLine{}
	for bi := range net.Brands {
		b := &net.Brands[bi]
		for ni := range b.Networks {
			n := &b.Networks[ni]
			g := &gross.Brands[bi].Networks[ni]
			allLines := budgetLines(n)
			allGross := budgetLines(g)
			for k, l := range budgetExtraLines(n) {
				allLines[k] = l
			}
			for k, l := range budgetExtraLines(g) {
				allGross[k] = l
			}
			for metric, line := range allLines {
				gl := allGross[metric]
				for q := 0; q < 4; q++ {
					cell := line.Q[q]
					l := repository.BudgetStoredLine{NetworkID: n.NetworkID, NetworkName: n.Brand, Brand: b.Brand, Quarter: q + 1, Metric: metric, Gross: gl.Q[q].A, Net: cell.A, Mark: cell.Mark, Source: "registry"}
					out = append(out, l)
					if cell.Edited != nil {
						l.Source = "manual"
						l.BaseNet = models.PtrFloat(cell.Edited.Base)
						l.BaseGross = models.PtrFloat(gl.Q[q].Edited.Base)
						l.Who = cell.Edited.Who
						l.When = cell.Edited.When
						out = append(out, l)
					}
				}
			}
		}
	}
	return out
}
func ValidBudgetSources(s models.BudgetSources) bool {
	if len(s.To) != 4 || len(s.Investments) != 4 {
		return false
	}
	for _, v := range s.To {
		if !validBudgetTurnoverSource(v) {
			return false
		}
	}
	for _, v := range s.Investments {
		if v != "plan" && v != "fact" && v != "forecast" {
			return false
		}
	}
	return true
}

func validBudgetTurnoverSource(v string) bool {
	return v == "plan" || v == "fact" || v == "forecast" || budgetOLAPSourceLine(v) != ""
}

func budgetOLAPSourceLine(v string) string {
	switch v {
	case "olap-ss":
		return "ss"
	case "olap-sswo":
		return "sswo"
	case "olap-pure":
		return "pure"
	case "olap-omni":
		return "omni"
	case "olap-mp":
		return "mp"
	default:
		return ""
	}
}

// BudgetOLAPSource reports whether a turnover source is an actual OLAP channel.
func BudgetOLAPSource(v string) bool { return budgetOLAPSourceLine(v) != "" }
func budgetLoad(f BudgetFilter, code string) (*models.BudgetResponse, []models.BudgetPromo, error) {
	if code == "PY" {
		f.Year--
		f.PastYear = true
		f.ToSources = [4]string{"fact", "fact", "fact", "fact"}
		f.InvestmentSources = f.ToSources
		r, p, err := loadBudgetDraft(f, nil)
		if err != nil {
			return nil, nil, err
		}
		r.Version = "PY"
		r.VersionInfo = models.BudgetVersion{Year: f.Year, Code: "PY", Name: "Факт прошлого года", Status: "frozen", Sources: models.BudgetSources{To: f.ToSources[:], Investments: f.InvestmentSources[:]}}
		budgetPresentation(r, f)
		return r, p, nil
	}
	if code == "" {
		code = "LIVE"
	}
	d, err := repository.LoadBudgetVersion(f.Year, code)
	if errors.Is(err, sql.ErrNoRows) && code == "LIVE" {
		d.Version = models.BudgetVersion{Year: f.Year, Code: "LIVE", Name: "Текущее", Status: "draft", Sources: models.BudgetSources{To: f.ToSources[:], Investments: f.InvestmentSources[:]}}
		err = nil
	}
	if err != nil {
		return nil, nil, err
	}
	if ValidBudgetSources(d.Version.Sources) {
		copy(f.ToSources[:], d.Version.Sources.To)
		copy(f.InvestmentSources[:], d.Version.Sources.Investments)
	}
	var r *models.BudgetResponse
	var promos []models.BudgetPromo
	if d.Version.Status == "frozen" {
		r = &models.BudgetResponse{}
		if err = json.Unmarshal([]byte(d.Metadata), r); err != nil {
			return nil, nil, err
		}
		r.Brands = []models.BudgetBrand{}
		budgetApplyLines(r, d.Lines, f)
		promos = []models.BudgetPromo{}
		for _, p := range d.Promos {
			if budgetTypeAllowed(r, f, p.NetworkID) {
				promos = append(promos, p)
			}
		}
	} else {
		r, promos, err = loadBudgetDraft(f, d.Promos)
		if err != nil {
			return nil, nil, err
		}
		budgetApplyLines(r, d.Lines, f)
	}
	budgetPresentation(r, f)
	r.Version = code
	r.VersionInfo = d.Version
	return r, promos, nil
}
func BudgetView(f BudgetFilter, code, compare, delta string) (*models.BudgetResponse, error) {
	r, _, err := budgetLoad(f, code)
	if err != nil {
		return nil, err
	}
	if compare != "" {
		other, _, err := budgetLoad(f, compare)
		if err != nil {
			return nil, err
		}
		budgetCompare(r, other, delta)
		r.Compare = compare
		r.CompareInfo = other.VersionInfo
		r.CompareStates = other.QuarterStates
	}
	return r, nil
}
func budgetDelta(a, b *float64, percent bool) *float64 {
	if a == nil || b == nil {
		return nil
	}
	if percent {
		if *b == 0 {
			return nil
		}
		return models.PtrFloat(round2((*a / *b - 1) * 100))
	}
	return models.PtrFloat(round2(*a - *b))
}
func budgetCompare(a, b *models.BudgetResponse, mode string) {
	compare := func(x, y *models.BudgetBrand) {
		xl, yl := budgetLines(x), budgetLines(y)
		xl["inv"] = &x.Investments
		yl["inv"] = &y.Investments
		xl["sales"] = &x.Sales
		yl["sales"] = &y.Sales
		xl["pct"] = &x.Pct
		yl["pct"] = &y.Pct
		for key, line := range xl {
			other := yl[key]
			for q := 0; q < 4; q++ {
				line.Q[q].B = other.Q[q].A
				line.Q[q].Delta = budgetDelta(line.Q[q].A, line.Q[q].B, mode == "pct" && key != "pct")
			}
			line.Year.B = other.Year.A
			line.Year.Delta = budgetDelta(line.Year.A, line.Year.B, mode == "pct" && key != "pct")
		}
	}
	empty := func(name string) models.BudgetBrand { return budgetBrand(name, 0, [4]budgetAmounts{}, BudgetFilter{}) }
	for _, other := range b.Brands {
		found := false
		for _, own := range a.Brands {
			if own.Brand == other.Brand {
				found = true
			}
		}
		if !found {
			a.Brands = append(a.Brands, empty(other.Brand))
		}
	}
	for i := range a.Brands {
		own := &a.Brands[i]
		other := empty(own.Brand)
		for _, row := range b.Brands {
			if row.Brand == own.Brand {
				other = row
				break
			}
		}
		compare(own, &other)
		for _, on := range other.Networks {
			found := false
			for _, n := range own.Networks {
				if n.NetworkID == on.NetworkID {
					found = true
				}
			}
			if !found {
				n := empty(on.Brand)
				n.NetworkID = on.NetworkID
				own.Networks = append(own.Networks, n)
			}
		}
		for j := range own.Networks {
			n := &own.Networks[j]
			on := empty(n.Brand)
			for _, row := range other.Networks {
				if row.NetworkID == n.NetworkID {
					on = row
					break
				}
			}
			compare(n, &on)
		}
	}
	compare(&a.Total, &b.Total)
}

// AllocateBudgetEdit rounds once per network and puts the remainder in the
// last row, preserving both the user-entered total and deterministic order.
func AllocateBudgetEdit(total float64, weights []float64) ([]float64, error) {
	sum := 0.0
	for _, v := range weights {
		sum += v
	}
	if sum == 0 {
		return nil, fmt.Errorf("Нет базовых значений для распределения по сетям")
	}
	result := make([]float64, len(weights))
	remaining := round2(total)
	for i, v := range weights {
		if i == len(weights)-1 {
			result[i] = remaining
		} else {
			result[i] = round2(total * v / sum)
			remaining = round2(remaining - result[i])
		}
	}
	return result, nil
}
func FreezeBudget(id int, stamp, who string) error {
	v, err := repository.BudgetVersionByID(id)
	if err != nil {
		return err
	}
	if v.Status != "draft" || v.UpdatedAt != stamp {
		return repository.ErrBudgetConflict
	}
	f := BudgetFilter{Year: v.Year}
	n, p, err := budgetLoad(f, v.Code)
	if err != nil {
		return err
	}
	f.Gross = true
	g, _, err := budgetLoad(f, v.Code)
	if err != nil {
		return err
	}
	meta := *n
	meta.Brands = nil
	meta.Total = models.BudgetBrand{}
	body, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	prices, err := repository.BudgetPricesAt(time.Now())
	if err != nil {
		return err
	}
	return repository.WriteBudget(repository.BudgetWrite{Prices: prices, ID: id, Expected: stamp, Who: who, Action: "freeze", Freeze: true, Metadata: string(body), Lines: budgetSnapshot(n, g), Promos: p})
}
func BudgetEditable(r *models.BudgetResponse, f BudgetFilter, role string) {
	reason := ""
	if role != "admin" && role != "analyst" {
		reason = "Доступно аналитику и администратору"
	} else if r.VersionInfo.Status != "draft" {
		reason = "Версия заморожена"
	} else if r.VersionInfo.ID == 0 {
		reason = "Сначала создайте версию"
	} else if len(f.NetworkTypes) > 0 || f.Gross || (f.Base != "" && f.Base != "reg-contract") {
		reason = "Правка доступна при всех типах сетей, в цене контракта и без НДС"
	}
	r.EditReason = reason
	r.CanEdit = reason == ""
	for i := range r.Brands {
		b := &r.Brands[i]
		for key, line := range budgetLines(b) {
			for q := 0; q < 4; q++ {
				line.Q[q].Editable = r.CanEdit && b.Brand != "Нераспределённый остаток пула" && !isClosedForecastMonth(r.Year, (q+1)*3, time.Now()) && (key == "to" || key == "gtnC" || key == "opexC")
			}
		}
		for q := 0; q < 4; q++ {
			b.Investments.Q[q].Editable = b.GtnContract.Q[q].Editable && (f.Source == "" || f.Source == "all") && (f.Kind == "" || f.Kind == "all")
		}
	}
}

type BudgetCellInput struct {
	Brand     string  `json:"brand"`
	Quarter   int     `json:"quarter"`
	Metric    string  `json:"metric"`
	Value     float64 `json:"value"`
	UpdatedAt string  `json:"updated_at"`
}

func SaveBudgetCell(id int, input BudgetCellInput, who string, f BudgetFilter, remove bool) error {
	if input.Quarter < 1 || input.Quarter > 4 || math.IsNaN(input.Value) || math.IsInf(input.Value, 0) {
		return fmt.Errorf("Некорректный квартал или сумма")
	}
	if input.Metric != "to" && input.Metric != "gtnC" && input.Metric != "opexC" && input.Metric != "inv" {
		return fmt.Errorf("Промо изменяются только выбором записей")
	}
	v, err := repository.BudgetVersionByID(id)
	if err != nil {
		return err
	}
	f.Year = v.Year
	r, _, err := budgetLoad(f, v.Code)
	if err != nil {
		return err
	}
	BudgetEditable(r, f, "analyst")
	if !r.CanEdit {
		return fmt.Errorf("%s", r.EditReason)
	}
	if isClosedForecastMonth(v.Year, input.Quarter*3, time.Now()) {
		return fmt.Errorf("Закрытый квартал нельзя изменять")
	}
	var b *models.BudgetBrand
	for i := range r.Brands {
		if r.Brands[i].Brand == input.Brand {
			b = &r.Brands[i]
		}
	}
	if b == nil || b.Brand == "Нераспределённый остаток пула" {
		return fmt.Errorf("Бренд недоступен для правки")
	}
	metric := input.Metric
	if metric == "inv" {
		if f.Source != "all" && f.Source != "" || f.Kind != "all" && f.Kind != "" {
			return fmt.Errorf("Сумма инвестиций правится при всех источниках и типах инвестиций")
		}
		metric = "gtnC"
	}
	if remove {
		return repository.WriteBudget(repository.BudgetWrite{ID: id, Expected: input.UpdatedAt, Who: who, Action: "reset_cell", RemoveBrand: b.Brand, RemoveQuarter: input.Quarter, RemoveMetric: metric})
	}
	q := input.Quarter - 1
	value := round2(input.Value)
	if input.Metric == "inv" {
		value = round2(value - valueOrZero(b.OpexContract.Q[q].A) - valueOrZero(b.GtnPromo.Q[q].A) - valueOrZero(b.OpexPromo.Q[q].A))
	}
	base := func(c models.BudgetCell) float64 {
		if c.Edited != nil {
			return c.Edited.Base
		}
		return valueOrZero(c.A)
	}
	weights := []float64{}
	for i := range b.Networks {
		weights = append(weights, base(budgetLines(&b.Networks[i])[metric].Q[q]))
	}
	amounts, err := AllocateBudgetEdit(value, weights)
	if err != nil {
		return err
	}
	grossFilter := f
	grossFilter.Gross = true
	gross, _, err := budgetLoad(grossFilter, v.Code)
	if err != nil {
		return err
	}
	var gb *models.BudgetBrand
	for i := range gross.Brands {
		if gross.Brands[i].Brand == b.Brand {
			gb = &gross.Brands[i]
		}
	}
	if gb == nil {
		return fmt.Errorf("Не удалось получить базу НДС")
	}
	lines := []repository.BudgetStoredLine{}
	for i := range b.Networks {
		n := &b.Networks[i]
		gn := &gb.Networks[i]
		add := func(m string, newNet float64) {
			old := budgetLines(n)[m].Q[q]
			gOld := budgetLines(gn)[m].Q[q]
			oldBase, grossBase := base(old), base(gOld)
			factor := 1.0
			if oldBase != 0 {
				factor = grossBase / oldBase
			} else if valueOrZero(n.OpexContract.Q[q].A) != 0 {
				factor = valueOrZero(gn.OpexContract.Q[q].A) / valueOrZero(n.OpexContract.Q[q].A)
			}
			if len(n.VATFactors) == 4 {
				factor = n.VATFactors[q]
			}
			newGross := round2(newNet * factor)
			if m == "to" {
				newGross = newNet
			}
			lines = append(lines, repository.BudgetStoredLine{NetworkID: n.NetworkID, NetworkName: n.Brand, Brand: b.Brand, Quarter: input.Quarter, Metric: m, Gross: models.PtrFloat(newGross), Net: models.PtrFloat(newNet), BaseGross: models.PtrFloat(grossBase), BaseNet: models.PtrFloat(oldBase), Source: "manual", Mark: old.Mark})
		}
		add(metric, amounts[i])
		if metric == "to" && base(n.To.Q[q]) != 0 && n.OlapTo.Q[q].A != nil {
			old := valueOrZero(n.OlapTo.Q[q].A)
			if n.OlapTo.Q[q].Edited != nil {
				old = n.OlapTo.Q[q].Edited.Base
			}
			newOlap := round2(old * amounts[i] / base(n.To.Q[q]))
			lines = append(lines, repository.BudgetStoredLine{NetworkID: n.NetworkID, NetworkName: n.Brand, Brand: b.Brand, Quarter: input.Quarter, Metric: "olap", Gross: models.PtrFloat(newOlap), Net: models.PtrFloat(newOlap), BaseGross: models.PtrFloat(old), BaseNet: models.PtrFloat(old), Source: "manual", Mark: n.To.Q[q].Mark})
		}
	}
	if metric == "to" {
		oldTO, oldGTN := 0.0, 0.0
		gtnWeights := []float64{}
		for i := range b.Networks {
			n := &b.Networks[i]
			oldTO += base(n.To.Q[q])
			gtn := base(n.GtnContract.Q[q])
			oldGTN += gtn
			gtnWeights = append(gtnWeights, gtn)
		}
		if oldTO == 0 {
			return fmt.Errorf("Нулевая база ТО: эффективная ставка не определена")
		}
		newGTN := round2(oldGTN / oldTO * value)
		gtnAmounts := make([]float64, len(b.Networks))
		if oldGTN != 0 {
			gtnAmounts, err = AllocateBudgetEdit(newGTN, gtnWeights)
			if err != nil {
				return err
			}
		}
		for i := range b.Networks {
			n, gn := &b.Networks[i], &gb.Networks[i]
			nb, gbv := base(n.GtnContract.Q[q]), base(gn.GtnContract.Q[q])
			factor := 1.0
			if nb != 0 {
				factor = gbv / nb
			}
			if len(n.VATFactors) == 4 {
				factor = n.VATFactors[q]
			}
			lines = append(lines, repository.BudgetStoredLine{NetworkID: n.NetworkID, NetworkName: n.Brand, Brand: b.Brand, Quarter: input.Quarter, Metric: "gtnC", Gross: models.PtrFloat(round2(gtnAmounts[i] * factor)), Net: models.PtrFloat(gtnAmounts[i]), BaseGross: models.PtrFloat(gbv), BaseNet: models.PtrFloat(nb), Source: "manual", Mark: n.GtnContract.Q[q].Mark})
		}
	}
	return repository.WriteBudget(repository.BudgetWrite{ID: id, Expected: input.UpdatedAt, Who: who, Action: "edit_cell", Lines: lines})
}

func budgetTypeAllowed(r *models.BudgetResponse, f BudgetFilter, id int) bool {
	if len(f.NetworkTypes) == 0 {
		return true
	}
	for _, kind := range f.NetworkTypes {
		for _, member := range r.TypeMembers[kind] {
			if member == id {
				return true
			}
		}
	}
	return false
}
