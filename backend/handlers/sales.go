package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"backend/config"
	"backend/models"

	"github.com/gin-gonic/gin"
)

func GetFilterOptions(c *gin.Context) {
	getDistinct := func(query string) []string {
		rows, e := config.DB.Query(query)
		if e != nil {
			return []string{}
		}
		defer rows.Close()
		var vals []string
		for rows.Next() {
			var v sql.NullString
			if err := rows.Scan(&v); err == nil && v.Valid && v.String != "" {
				vals = append(vals, v.String)
			}
		}
		return vals
	}

	result := gin.H{
		"brandName":   getDistinct("SELECT DISTINCT brandName FROM dbo.tbl_EcomSalesConsolidated WHERE brandName IS NOT NULL ORDER BY brandName"),
		"networkName": getDistinct("SELECT DISTINCT networkName FROM dbo.tbl_EcomSalesConsolidated WHERE networkName IS NOT NULL ORDER BY networkName"),
		"un_rub":      getDistinct("SELECT DISTINCT un_rub FROM dbo.tbl_ChannelSegmentMapping WHERE un_rub IS NOT NULL ORDER BY un_rub"),
		"segment":     getDistinct("SELECT DISTINCT segment FROM dbo.tbl_ChannelSegmentMapping WHERE segment IS NOT NULL ORDER BY segment"),
		"channel":     getDistinct("SELECT DISTINCT channel FROM dbo.tbl_ChannelSegmentMapping WHERE channel IS NOT NULL ORDER BY channel"),
	}

	mappingQuery := `SELECT segment, channel FROM dbo.tbl_ChannelSegmentMapping WHERE segment IS NOT NULL AND channel IS NOT NULL GROUP BY segment, channel ORDER BY segment, channel`
	rows, e := config.DB.Query(mappingQuery)
	if e != nil {
		result["segmentChannelMap"] = make(map[string][]string)
		result["channelSegmentMap"] = make(map[string][]string)
	} else {
		defer rows.Close()
		segChanMap := make(map[string][]string)
		chanSegMap := make(map[string][]string)
		for rows.Next() {
			var seg, chanVal sql.NullString
			if err := rows.Scan(&seg, &chanVal); err == nil {
				if seg.Valid && chanVal.Valid && seg.String != "" && chanVal.String != "" {
					segChanMap[seg.String] = append(segChanMap[seg.String], chanVal.String)
					chanSegMap[chanVal.String] = append(chanSegMap[chanVal.String], seg.String)
				}
			}
		}
		result["segmentChannelMap"] = segChanMap
		result["channelSegmentMap"] = chanSegMap
	}
	c.JSON(http.StatusOK, result)
}

func GetData(c *gin.Context) {
	yearFromStr := c.Query("yearFrom")
	yearToStr := c.Query("yearTo")
	months := c.QueryArray("months")
	brandNames := c.QueryArray("brandName")
	networkNames := c.QueryArray("networkName")
	unRubs := c.QueryArray("un_rub")
	segments := c.QueryArray("segment")
	channels := c.QueryArray("channel")

	query := `;WITH UnpivotedData AS (SELECT id, [year], [month], brandName, productName, networkName, updated_at, CAST(metric_type AS NVARCHAR(256)) AS metric_type, metric_value FROM dbo.tbl_EcomSalesConsolidated UNPIVOT (metric_value FOR metric_type IN (qty, rub, qty_ZC, rub_ZC, qty_PLZ, rub_PLZ, qty_AR, rub_AR, qty_OZ, rub_OZ, qty_EA, rub_EA, qty_PMP, rub_PMP, qty_OMNI, rub_OMNI, qty_NW, rub_NW, NW_wo_ecom, SS_wo_ecom, rub_NW_wo_ecom, rub_SS_wo_ecom)) AS unpvt WHERE 1=1`
	args := []interface{}{}

	if yearFromStr != "" {
		if y, _ := strconv.Atoi(yearFromStr); true {
			query += " AND [year] >= ?"
			args = append(args, y)
		}
	}
	if yearToStr != "" {
		if y, _ := strconv.Atoi(yearToStr); true {
			query += " AND [year] <= ?"
			args = append(args, y)
		}
	}
	if len(months) > 0 {
		placeholders := make([]string, 0, len(months))
		for _, m := range months {
			if val, _ := strconv.Atoi(m); true {
				placeholders = append(placeholders, "?")
				args = append(args, val)
			}
		}
		if len(placeholders) > 0 {
			query += " AND [month] IN (" + strings.Join(placeholders, ",") + ")"
		}
	}
	if len(brandNames) > 0 {
		conds := make([]string, 0, len(brandNames))
		for _, v := range brandNames {
			if v != "" {
				conds = append(conds, "brandName LIKE ?")
				args = append(args, "%"+v+"%")
			}
		}
		if len(conds) > 0 {
			query += " AND (" + strings.Join(conds, " OR ") + ")"
		}
	}
	if len(networkNames) > 0 {
		conds := make([]string, 0, len(networkNames))
		for _, v := range networkNames {
			if v != "" {
				conds = append(conds, "networkName LIKE ?")
				args = append(args, "%"+v+"%")
			}
		}
		if len(conds) > 0 {
			query += " AND (" + strings.Join(conds, " OR ") + ")"
		}
	}

	query += `) SELECT f.id, f.[year], f.[month], f.brandName, f.productName, f.networkName, f.metric_type, f.metric_value, m.un_rub, m.segment, m.channel, f.updated_at FROM UnpivotedData f LEFT JOIN dbo.tbl_ChannelSegmentMapping m WITH (INDEX(IX_tbl_ChannelSegmentMapping_name)) ON f.metric_type = m.name WHERE f.metric_value != 0 AND f.metric_value IS NOT NULL`

	appendFilter := func(col string, values []string) {
		if len(values) > 0 {
			placeholders := make([]string, 0, len(values))
			for _, v := range values {
				if v != "" {
					placeholders = append(placeholders, "?")
					args = append(args, v)
				}
			}
			if len(placeholders) > 0 {
				query += " AND " + col + " IN (" + strings.Join(placeholders, ",") + ")"
			}
		}
	}
	appendFilter("m.un_rub", unRubs)
	appendFilter("m.segment", segments)
	appendFilter("m.channel", channels)

	all := c.Query("all")
	if all == "true" {
		query += " ORDER BY f.[year] DESC, f.[month] ASC, f.metric_type OPTION (RECOMPILE)"
	} else {
		offset, limit := 0, 1000
		if p := c.Query("page"); p != "" {
			if page, _ := strconv.Atoi(p); page > 0 {
				if l := c.Query("limit"); l != "" {
					if lim, _ := strconv.Atoi(l); lim > 0 {
						offset = (page - 1) * lim
						limit = lim
					}
				}
			}
		}
		query += " ORDER BY f.[year] DESC, f.[month] ASC, f.metric_type OFFSET ? ROWS FETCH NEXT ? ROWS ONLY OPTION (RECOMPILE)"
		args = append(args, offset, limit)
	}

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed"})
		return
	}
	defer rows.Close()

	var results []models.Row
	for rows.Next() {
		var r models.Row
		if err := rows.Scan(&r.ID, &r.Year, &r.Month, &r.BrandName, &r.ProductName, &r.NetworkName, &r.MetricType, &r.MetricValue, &r.UnRub, &r.Segment, &r.Channel, &r.UpdatedAt); err != nil {
			continue
		}
		results = append(results, r)
	}
	c.JSON(http.StatusOK, gin.H{"data": results})
}

func GetDrilldown(c *gin.Context) {
	brandName := c.Query("brandName")
	networkName := c.Query("networkName")
	if brandName == "" || networkName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "brandName и networkName обязательны"})
		return
	}

	yearFromStr := c.Query("yearFrom")
	yearToStr := c.Query("yearTo")
	months := c.QueryArray("months")
	segments := c.QueryArray("segment")
	channels := c.QueryArray("channel")

	query := `;WITH UnpivotedData AS (SELECT id, [year], [month], brandName, productName, networkName, updated_at, CAST(metric_type AS NVARCHAR(256)) AS metric_type, metric_value FROM dbo.tbl_EcomSalesConsolidated UNPIVOT (metric_value FOR metric_type IN (qty, rub, qty_ZC, rub_ZC, qty_PLZ, rub_PLZ, qty_AR, rub_AR, qty_OZ, rub_OZ, qty_EA, rub_EA, qty_PMP, rub_PMP, qty_OMNI, rub_OMNI, qty_NW, rub_NW, NW_wo_ecom, SS_wo_ecom, rub_NW_wo_ecom, rub_SS_wo_ecom)) AS unpvt WHERE brandName = ? AND networkName = ?`
	args := []interface{}{brandName, networkName}

	if yearFromStr != "" {
		if y, _ := strconv.Atoi(yearFromStr); true {
			query += " AND [year] >= ?"
			args = append(args, y)
		}
	}
	if yearToStr != "" {
		if y, _ := strconv.Atoi(yearToStr); true {
			query += " AND [year] <= ?"
			args = append(args, y)
		}
	}
	if len(months) > 0 {
		placeholders := make([]string, 0, len(months))
		for _, m := range months {
			if val, _ := strconv.Atoi(m); true {
				placeholders = append(placeholders, "?")
				args = append(args, val)
			}
		}
		if len(placeholders) > 0 {
			query += " AND [month] IN (" + strings.Join(placeholders, ",") + ")"
		}
	}

	query += `) SELECT f.[year], f.[month], f.metric_type, SUM(f.metric_value) as total_value, m.un_rub, m.segment, m.channel FROM UnpivotedData f LEFT JOIN dbo.tbl_ChannelSegmentMapping m WITH (INDEX(IX_tbl_ChannelSegmentMapping_name)) ON f.metric_type = m.name WHERE f.metric_value != 0 AND f.metric_value IS NOT NULL`

	appendFilter := func(col string, values []string) {
		if len(values) > 0 {
			placeholders := make([]string, 0, len(values))
			for _, v := range values {
				if v != "" {
					placeholders = append(placeholders, "?")
					args = append(args, v)
				}
			}
			if len(placeholders) > 0 {
				query += " AND " + col + " IN (" + strings.Join(placeholders, ",") + ")"
			}
		}
	}
	appendFilter("m.segment", segments)
	appendFilter("m.channel", channels)

	query += " GROUP BY f.[year], f.[month], f.metric_type, m.un_rub, m.segment, m.channel ORDER BY f.[year] DESC, f.[month] ASC, f.metric_type OPTION (RECOMPILE)"

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed"})
		return
	}
	defer rows.Close()

	var results []models.DrilldownRow
	for rows.Next() {
		var r models.DrilldownRow
		if err := rows.Scan(&r.Year, &r.Month, &r.MetricType, &r.TotalValue, &r.UnRub, &r.Segment, &r.Channel); err != nil {
			continue
		}
		results = append(results, r)
	}
	c.JSON(http.StatusOK, gin.H{"brandName": brandName, "networkName": networkName, "data": results})
}
