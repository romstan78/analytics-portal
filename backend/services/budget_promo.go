package services

import (
	"fmt"
	"sort"
	"time"

	"backend/models"
	"backend/repository"
)

func BudgetPromosView(f BudgetFilter, code, compare, brand string, quarter int, status, kind string, page int) (*models.BudgetPromoResponse, error) {
	r, p, err := budgetLoad(f, code)
	if err != nil {
		return nil, err
	}
	other := map[int]models.BudgetPromo{}
	if compare != "" {
		_, rows, err := budgetLoad(f, compare)
		if err != nil {
			return nil, err
		}
		for _, v := range rows {
			other[v.ID] = v
		}
	}
	current := map[int]bool{}
	for _, v := range p {
		current[v.ID] = true
	}
	for _, v := range other {
		if !current[v.ID] {
			v.Included = false
			v.Change = "снято"
			p = append(p, v)
		}
	}
	out := &models.BudgetPromoResponse{Data: []models.BudgetPromo{}, UpdatedAt: r.VersionInfo.UpdatedAt}
	amount := func(v models.BudgetPromo) float64 {
		if !v.Included {
			return 0
		}
		if f.Gross {
			return v.BudgetRub
		}
		return v.BudgetNet
	}
	for _, v := range p {
		if (brand != "" && v.Brand != brand) || (quarter > 0 && v.Quarter != quarter) || (status != "" && v.Status != status) {
			continue
		}
		typ := "gtn"
		if isOPEXInvestment(&v.Type) {
			typ = "opex"
		}
		if kind != "" && kind != typ {
			continue
		}
		a := amount(v)
		v.A = &a
		if compare != "" {
			old, ok := other[v.ID]
			b := amount(old)
			v.B = &b
			v.Delta = budgetDelta(v.A, v.B, false)
			if v.Change != "снято" {
				switch {
				case !v.Included:
					v.Change = "исключено"
				case !ok:
					v.Change = "новое"
				case v.FactRub > 0 && old.FactRub <= 0:
					v.Change = "факт вместо плана"
				case a != b || v.Status != old.Status:
					v.Change = "изменено"
				}
			}
		}
		out.Data = append(out.Data, v)
		out.Sum = round2(out.Sum + a)
	}
	sort.Slice(out.Data, func(i, j int) bool {
		a, b := valueOrZero(out.Data[i].A), valueOrZero(out.Data[j].A)
		if a != b {
			return a > b
		}
		return out.Data[i].ID < out.Data[j].ID
	})
	out.Total = len(out.Data)
	top := 0.0
	for i := 0; i < len(out.Data) && i < 10; i++ {
		top += valueOrZero(out.Data[i].A)
	}
	if out.Sum != 0 {
		out.Top10Pct = models.PtrFloat(round2(top / out.Sum * 100))
	}
	if page < 1 {
		page = 1
	}
	start := (page - 1) * 50
	if start > len(out.Data) {
		start = len(out.Data)
	}
	end := start + 50
	if end > len(out.Data) {
		end = len(out.Data)
	}
	out.Data = out.Data[start:end]
	return out, nil
}
func SaveBudgetPromos(id int, ids []int, included bool, stamp, who string) error {
	v, err := repository.BudgetVersionByID(id)
	if err != nil {
		return err
	}
	if v.Status != "draft" || v.UpdatedAt != stamp {
		return repository.ErrBudgetConflict
	}
	_, rows, err := budgetLoad(BudgetFilter{Year: v.Year}, v.Code)
	if err != nil {
		return err
	}
	selected := map[int]bool{}
	for _, id := range ids {
		selected[id] = true
	}
	changes := []models.BudgetPromo{}
	for _, p := range rows {
		if !selected[p.ID] {
			continue
		}
		if p.Status == "исключено" && included {
			return fmt.Errorf("Отклонённое или отменённое промо нельзя включить")
		}
		if isClosedForecastMonth(v.Year, p.Quarter*3, time.Now()) {
			return fmt.Errorf("Состав закрытого квартала нельзя изменять")
		}
		p.Included = included
		changes = append(changes, p)
		delete(selected, p.ID)
	}
	if len(selected) > 0 {
		return fmt.Errorf("Промо не найдено в версии")
	}
	return repository.WriteBudget(repository.BudgetWrite{ID: id, Expected: stamp, Who: who, Action: "select_promos", Promos: changes})
}
