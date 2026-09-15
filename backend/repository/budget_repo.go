package repository

import (
	"encoding/json"
	"errors"
	"time"

	"backend/config"
	"backend/models"

	mssql "github.com/microsoft/go-mssqldb"
)

// BudgetNetworkTypes uses geographical network types, not contract types.
// DISTINCT prevents duplicate mappings from multiplying financial rows.
func BudgetNetworkTypes() (map[int][]string, error) {
	rows, err := config.DB.Query(`SELECT DISTINCT n.id, COALESCE(NULLIF(LTRIM(RTRIM(g.network_type)),N''),N'(не задан)')
		FROM dbo.tbl_Networks n LEFT JOIN dbo.tbl_NetworkGeoMapping g ON g.network_name=n.name
		WHERE n.is_active=1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[int][]string{}
	for rows.Next() {
		var id int
		var kind string
		if err := rows.Scan(&id, &kind); err != nil {
			return nil, err
		}
		result[id] = append(result[id], kind)
	}
	return result, rows.Err()
}

// BudgetStoredLine is one network/brand/quarter measure, in both VAT bases.
type BudgetStoredLine struct {
	NetworkID                      int
	NetworkName, Brand             string
	Quarter                        int
	Metric                         string
	Gross, Net, BaseGross, BaseNet *float64
	Mark, Source, Who, When        string
}

type BudgetVersionData struct {
	Version  models.BudgetVersion
	Metadata string
	Lines    []BudgetStoredLine
	Promos   []models.BudgetPromo
}

var ErrBudgetVersionExists = errors.New("Версия с таким кодом за этот год уже существует")

var ErrBudgetConflict = errors.New("Версия изменилась или уже заморожена. Обновите бюджет")

func budgetScanVersion(row interface{ Scan(...interface{}) error }) (models.BudgetVersion, string, error) {
	var v models.BudgetVersion
	var sources, metadata string
	err := row.Scan(&v.ID, &v.Year, &v.Code, &v.Name, &v.Status, &v.FrozenAt, &v.CreatedBy, &v.UpdatedAt, &sources, &metadata)
	if err == nil {
		err = json.Unmarshal([]byte(sources), &v.Sources)
	}
	return v, metadata, err
}

const budgetVersionColumns = `id,[year],code,name,status,ISNULL(CONVERT(varchar(33),frozen_at,126),''),created_by,CONVERT(varchar(33),updated_at,126),sources,ISNULL(metadata,'')`

func BudgetVersions(year int) ([]models.BudgetVersion, error) {
	rows, err := config.DB.Query(`SELECT `+budgetVersionColumns+` FROM dbo.tbl_BudgetVersions WHERE [year]=? ORDER BY CASE code WHEN 'B' THEN 0 WHEN 'F1' THEN 1 WHEN 'F2' THEN 2 WHEN 'F3' THEN 3 ELSE 4 END`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []models.BudgetVersion{}
	for rows.Next() {
		v, _, err := budgetScanVersion(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
func BudgetVersionByID(id int) (models.BudgetVersion, error) {
	v, _, err := budgetScanVersion(config.DB.QueryRow(`SELECT `+budgetVersionColumns+` FROM dbo.tbl_BudgetVersions WHERE id=?`, id))
	return v, err
}
func CreateBudgetVersion(year int, code, name, who string, sources models.BudgetSources) (models.BudgetVersion, error) {
	body, err := json.Marshal(sources)
	if err != nil {
		return models.BudgetVersion{}, err
	}
	var id int
	err = config.DB.QueryRow(`INSERT dbo.tbl_BudgetVersions([year],code,name,created_by,sources) OUTPUT INSERTED.id VALUES(?,?,?,?,?)`, year, code, name, who, string(body)).Scan(&id)
	if err != nil {
		var dbError mssql.Error
		if errors.As(err, &dbError) && (dbError.Number == 2601 || dbError.Number == 2627) {
			return models.BudgetVersion{}, ErrBudgetVersionExists
		}
		return models.BudgetVersion{}, err
	}
	return BudgetVersionByID(id)
}
func LoadBudgetVersion(year int, code string) (BudgetVersionData, error) {
	var d BudgetVersionData
	var err error
	d.Version, d.Metadata, err = budgetScanVersion(config.DB.QueryRow(`SELECT `+budgetVersionColumns+` FROM dbo.tbl_BudgetVersions WHERE [year]=? AND code=?`, year, code))
	if err != nil {
		return d, err
	}
	rows, err := config.DB.Query(`SELECT network_id,network_name,brand_as,quarter,metric,value_rub,value_net,base_rub,base_net,mark,source,updated_by,CONVERT(varchar(33),updated_at,126) FROM dbo.tbl_BudgetLines WHERE version_id=? ORDER BY network_id,brand_as,quarter,metric,CASE source WHEN 'registry' THEN 0 ELSE 1 END`, d.Version.ID)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var l BudgetStoredLine
		err = rows.Scan(&l.NetworkID, &l.NetworkName, &l.Brand, &l.Quarter, &l.Metric, &l.Gross, &l.Net, &l.BaseGross, &l.BaseNet, &l.Mark, &l.Source, &l.Who, &l.When)
		if err != nil {
			rows.Close()
			return d, err
		}
		d.Lines = append(d.Lines, l)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return d, err
	}
	rows, err = config.DB.Query(`SELECT promo_id,network_id,network_name,brand_as,quarter,month,gtn_opex,status_norm,mechanics,plan_rub,fact_rub,budget_rub,budget_net,included FROM dbo.tbl_BudgetPromoLines WHERE version_id=?`, d.Version.ID)
	if err != nil {
		return d, err
	}
	defer rows.Close()
	for rows.Next() {
		var p models.BudgetPromo
		if err = rows.Scan(&p.ID, &p.NetworkID, &p.Network, &p.Brand, &p.Quarter, &p.Month, &p.Type, &p.Status, &p.Mechanics, &p.PlanRub, &p.FactRub, &p.BudgetRub, &p.BudgetNet, &p.Included); err != nil {
			return d, err
		}
		d.Promos = append(d.Promos, p)
	}
	return d, rows.Err()
}
func BudgetPromos(year int) ([]models.BudgetPromo, error) {
	rows, err := config.DB.Query(`SELECT p.id,n.id,n.name,COALESCE(NULLIF(LTRIM(RTRIM(p.brand_as)),N''),N'Без бренда'),(p.[month]-1)/3+1,p.[month],ISNULL(p.gtn_opex,''),COALESCE(s.status_norm,N'черновик'),ISNULL(p.mechanics,''),ISNULL(p.plan_investments_rub,0),ISNULL(p.actual_investments,0)
 FROM dbo.tbl_PromoActivities p JOIN dbo.tbl_Networks n ON n.name=p.network_name
 OUTER APPLY (SELECT TOP 1 m.status_norm FROM dbo.tbl_BudgetPromoStatusMap m
 WHERE (m.raw_status=N'*' OR m.raw_status=LOWER(LTRIM(RTRIM(ISNULL(p.status,''))))) AND
 (m.agreement_rule='any' OR (m.agreement_rule='rejected' AND (p.agreement1_status='rejected' OR p.agreement2_status='rejected')) OR
 (m.agreement_rule='approved' AND p.agreement1_status='approved' AND p.agreement2_status='approved') OR
 (m.agreement_rule='pending' AND (p.agreement1_status IN ('approved','commented') OR p.agreement2_status IN ('approved','commented'))))
 ORDER BY m.priority DESC,m.raw_status,m.agreement_rule) s
 WHERE p.[year]=? AND p.deleted_at IS NULL AND n.is_active=1 AND p.[month] BETWEEN 1 AND 12`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []models.BudgetPromo{}
	for rows.Next() {
		var p models.BudgetPromo
		if err := rows.Scan(&p.ID, &p.NetworkID, &p.Network, &p.Brand, &p.Quarter, &p.Month, &p.Type, &p.Status, &p.Mechanics, &p.PlanRub, &p.FactRub); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

type BudgetWrite struct {
	Sources                   *models.BudgetSources
	Prices                    []BudgetPrice
	ID                        int
	Expected, Who, Action     string
	Freeze                    bool
	Metadata                  string
	Lines                     []BudgetStoredLine
	Promos                    []models.BudgetPromo
	RemoveBrand, RemoveMetric string
	RemoveQuarter             int
}

func WriteBudget(w BudgetWrite) error {
	tx, err := config.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var stamp, status string
	err = tx.QueryRow(`SELECT CONVERT(varchar(33),updated_at,126),status FROM dbo.tbl_BudgetVersions WITH(UPDLOCK,HOLDLOCK) WHERE id=?`, w.ID).Scan(&stamp, &status)
	if err != nil {
		return err
	}
	if stamp != w.Expected || status != "draft" {
		return ErrBudgetConflict
	}
	if w.Freeze {
		if _, err = tx.Exec(`DELETE FROM dbo.tbl_BudgetLines WHERE version_id=?`, w.ID); err != nil {
			return err
		}
		if _, err = tx.Exec(`DELETE FROM dbo.tbl_BudgetPromoLines WHERE version_id=?`, w.ID); err != nil {
			return err
		}
	}
	if w.RemoveMetric != "" {
		if _, err = tx.Exec(`DELETE FROM dbo.tbl_BudgetLines WHERE version_id=? AND brand_as=? AND quarter=? AND (metric=? OR (?='to' AND metric IN ('gtnC','olap'))) AND source='manual'`, w.ID, w.RemoveBrand, w.RemoveQuarter, w.RemoveMetric, w.RemoveMetric); err != nil {
			return err
		}
	}
	for _, l := range w.Lines {
		if _, err = tx.Exec(`DELETE FROM dbo.tbl_BudgetLines WHERE version_id=? AND network_id=? AND brand_as=? AND quarter=? AND metric=? AND source=?`, w.ID, l.NetworkID, l.Brand, l.Quarter, l.Metric, l.Source); err != nil {
			return err
		}
		who, when := w.Who, ""
		if w.Freeze && l.Source == "manual" {
			who, when = l.Who, l.When
		}
		if _, err = tx.Exec(`INSERT dbo.tbl_BudgetLines(version_id,network_id,network_name,brand_as,quarter,metric,value_rub,value_net,base_rub,base_net,mark,source,updated_by,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,COALESCE(CONVERT(datetime2(7),NULLIF(?,'')),SYSUTCDATETIME()))`, w.ID, l.NetworkID, l.NetworkName, l.Brand, l.Quarter, l.Metric, l.Gross, l.Net, l.BaseGross, l.BaseNet, l.Mark, l.Source, who, when); err != nil {
			return err
		}
	}
	for _, p := range w.Promos {
		if _, err = tx.Exec(`DELETE FROM dbo.tbl_BudgetPromoLines WHERE version_id=? AND promo_id=?`, w.ID, p.ID); err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT dbo.tbl_BudgetPromoLines(version_id,promo_id,network_id,network_name,brand_as,quarter,month,gtn_opex,status_norm,mechanics,plan_rub,fact_rub,budget_rub,budget_net,included,included_by) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, w.ID, p.ID, p.NetworkID, p.Network, p.Brand, p.Quarter, p.Month, p.Type, p.Status, p.Mechanics, p.PlanRub, p.FactRub, p.BudgetRub, p.BudgetNet, p.Included, w.Who); err != nil {
			return err
		}
	}
	if w.Sources != nil {
		body, e := json.Marshal(w.Sources)
		if e != nil {
			return e
		}
		if _, e = tx.Exec(`UPDATE dbo.tbl_BudgetVersions SET sources=? WHERE id=?`, string(body), w.ID); e != nil {
			return e
		}
	}
	if w.Freeze {
		for _, p := range w.Prices {
			if _, e := tx.Exec(`INSERT dbo.tbl_BudgetOlapPrices(version_id,sku,price,source_year,source_month) VALUES(?,?,?,?,?)`, w.ID, p.SKU, p.Price, p.Year, p.Month); e != nil {
				return e
			}
		}
	}
	if w.Freeze {
		_, err = tx.Exec(`UPDATE dbo.tbl_BudgetVersions SET status='frozen',frozen_at=SYSUTCDATETIME(),metadata=?,updated_at=DATEADD(nanosecond,100,SYSUTCDATETIME()) WHERE id=?`, w.Metadata, w.ID)
	} else {
		_, err = tx.Exec(`UPDATE dbo.tbl_BudgetVersions SET updated_at=DATEADD(nanosecond,100,SYSUTCDATETIME()) WHERE id=?`, w.ID)
	}
	if err != nil {
		return err
	}
	details, _ := json.Marshal(w)
	_, err = tx.Exec(`INSERT dbo.tbl_AuditLog(entity_type,entity_id,user_name,action_type,changed_fields) VALUES('budget',?,?,?,?)`, w.ID, w.Who, w.Action, string(details))
	if err != nil {
		return err
	}
	return tx.Commit()
}

type BudgetPrice struct {
	Brand, SKU  string
	Price       float64
	Year, Month int
}

// BudgetPricesAt fixes the latest available OLAP month no later than the
// snapshot date, including budgets for a future year. Generated future demo
// sales must never move the snapshot price into a future month.
func BudgetPricesAt(now time.Time) ([]BudgetPrice, error) {
	rows, err := config.DB.Query(`WITH latest AS (
 SELECT TOP 1 [year],[month] FROM dbo.tbl_EcomSalesNormalized
 WHERE segment=N'OLAP SS' AND ([year]<? OR ([year]=? AND [month]<=?))
 GROUP BY [year],[month]
 HAVING SUM(CASE WHEN un_rub=N'руб' THEN metric_value ELSE 0 END)>0
 AND SUM(CASE WHEN un_rub=N'уп' THEN metric_value ELSE 0 END)>0
 ORDER BY [year] DESC,[month] DESC)
 SELECT COALESCE(MAX(m.brand_as),N'Без бренда'),LTRIM(RTRIM(s.productName)),
 SUM(CASE WHEN s.un_rub=N'руб' THEN s.metric_value ELSE 0 END)/NULLIF(SUM(CASE WHEN s.un_rub=N'уп' THEN s.metric_value ELSE 0 END),0),s.[year],s.[month]
 FROM dbo.tbl_EcomSalesNormalized s JOIN latest l ON l.[year]=s.[year] AND l.[month]=s.[month]
 OUTER APPLY(SELECT MAX(NULLIF(LTRIM(RTRIM(brand_as)),N'')) brand_as FROM dbo.tbl_SKUMapping WHERE LTRIM(RTRIM(sku))=LTRIM(RTRIM(s.productName))) m
 WHERE s.segment=N'OLAP SS' AND NULLIF(LTRIM(RTRIM(s.productName)),N'') IS NOT NULL
 GROUP BY LTRIM(RTRIM(s.productName)),s.[year],s.[month]
 HAVING SUM(CASE WHEN s.un_rub=N'руб' THEN s.metric_value ELSE 0 END)>0 AND SUM(CASE WHEN s.un_rub=N'уп' THEN s.metric_value ELSE 0 END)>0
 ORDER BY LTRIM(RTRIM(s.productName))`, now.Year(), now.Year(), int(now.Month()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []BudgetPrice{}
	for rows.Next() {
		var p BudgetPrice
		if err := rows.Scan(&p.Brand, &p.SKU, &p.Price, &p.Year, &p.Month); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

type BudgetSale struct {
	NetworkID, Month        int
	Brand, Segment, Channel string
	Rub                     float64
}

func BudgetSales(year int) ([]BudgetSale, error) {
	rows, err := config.DB.Query(`SELECT n.id,s.[month],COALESCE(m.brand_as,N'Без бренда'),ISNULL(s.segment,''),ISNULL(s.channel,''),SUM(s.metric_value)
 FROM dbo.tbl_EcomSalesNormalized s JOIN dbo.tbl_Networks n ON n.name=s.networkName AND n.is_active=1
 OUTER APPLY (SELECT MAX(NULLIF(LTRIM(RTRIM(brand_as)),N'')) brand_as FROM dbo.tbl_SKUMapping WHERE LTRIM(RTRIM(sku))=LTRIM(RTRIM(s.productName))) m
 WHERE s.[year]=? AND s.un_rub=N'руб' AND s.[month] BETWEEN 1 AND 12
 GROUP BY n.id,s.[month],m.brand_as,s.segment,s.channel`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []BudgetSale{}
	for rows.Next() {
		var s BudgetSale
		if err := rows.Scan(&s.NetworkID, &s.Month, &s.Brand, &s.Segment, &s.Channel, &s.Rub); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}
