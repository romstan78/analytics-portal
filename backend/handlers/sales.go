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
		"brandName":   getDistinct("SELECT DISTINCT brandName FROM dbo.tbl_EcomSalesNormalized WHERE brandName IS NOT NULL ORDER BY brandName"),
		"networkName": getDistinct("SELECT DISTINCT networkName FROM dbo.tbl_EcomSalesNormalized WHERE networkName IS NOT NULL ORDER BY networkName"),
		"un_rub":      getDistinct("SELECT DISTINCT un_rub FROM dbo.tbl_EcomSalesNormalized WHERE un_rub IS NOT NULL ORDER BY un_rub"),
		"segment":     getDistinct("SELECT DISTINCT segment FROM dbo.tbl_EcomSalesNormalized WHERE segment IS NOT NULL ORDER BY segment"),
		"channel":     getDistinct("SELECT DISTINCT channel FROM dbo.tbl_EcomSalesNormalized WHERE channel IS NOT NULL ORDER BY channel"),
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

	baseWhere := " WHERE n.metric_value != 0 AND n.metric_value IS NOT NULL"
	baseSelect := "SELECT n.id, n.[year], n.[month], n.brandName, n.productName, n.networkName, n.metric_type, n.metric_value, n.un_rub, n.segment, n.channel, n.updated_at FROM dbo.tbl_EcomSalesNormalized n"
	args := []interface{}{}

	if yearFromStr != "" {
		if y, _ := strconv.Atoi(yearFromStr); true {
			baseWhere += " AND n.[year] >= ?"
			args = append(args, y)
		}
	}
	if yearToStr != "" {
		if y, _ := strconv.Atoi(yearToStr); true {
			baseWhere += " AND n.[year] <= ?"
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
			baseWhere += " AND n.[month] IN (" + strings.Join(placeholders, ",") + ")"
		}
	}
	if len(brandNames) > 0 {
		conds := make([]string, 0, len(brandNames))
		for _, v := range brandNames {
			if v != "" {
				conds = append(conds, "n.brandName LIKE ?")
				args = append(args, "%"+v+"%")
			}
		}
		if len(conds) > 0 {
			baseWhere += " AND (" + strings.Join(conds, " OR ") + ")"
		}
	}
	if len(networkNames) > 0 {
		conds := make([]string, 0, len(networkNames))
		for _, v := range networkNames {
			if v != "" {
				conds = append(conds, "n.networkName LIKE ?")
				args = append(args, "%"+v+"%")
			}
		}
		if len(conds) > 0 {
			baseWhere += " AND (" + strings.Join(conds, " OR ") + ")"
		}
	}

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
				baseWhere += " AND " + col + " IN (" + strings.Join(placeholders, ",") + ")"
			}
		}
	}
	appendFilter("n.un_rub", unRubs)
	appendFilter("n.segment", segments)
	appendFilter("n.channel", channels)

	all := c.Query("all")

	if all == "true" {
		// Экспорт — возвращаем всё
		query := baseSelect + baseWhere + " ORDER BY n.[year] DESC, n.[month] ASC, n.metric_type"
		rows, err := config.DB.Query(query, args...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed", "data": []interface{}{}})
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
		if results == nil {
			results = []models.Row{}
		}
		c.JSON(http.StatusOK, gin.H{"data": results})
		return
	}

	// Пагинация
	countQuery := "SELECT COUNT(*) FROM dbo.tbl_EcomSalesNormalized n" + baseWhere
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)

	var totalRows int
	if err := config.DB.QueryRow(countQuery, countArgs...).Scan(&totalRows); err != nil {
		totalRows = 0
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "100"))
	if pageSize <= 0 {
		pageSize = 100
	}
	if pageSize > 1000 {
		pageSize = 1000
	}
	offset := page * pageSize

	query := baseSelect + baseWhere + " ORDER BY n.[year] DESC, n.[month] ASC, n.metric_type OFFSET ? ROWS FETCH NEXT ? ROWS ONLY"
	args = append(args, offset, pageSize)

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed", "data": []interface{}{}})
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
	if results == nil {
		results = []models.Row{}
	}
	c.JSON(http.StatusOK, gin.H{"data": results, "totalRows": totalRows})
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

	query := `SELECT n.[year], n.[month], n.metric_type, SUM(n.metric_value) as total_value, n.un_rub, n.segment, n.channel FROM dbo.tbl_EcomSalesNormalized n WHERE n.brandName = ? AND n.networkName = ? AND n.metric_value != 0 AND n.metric_value IS NOT NULL`
	args := []interface{}{brandName, networkName}

	if yearFromStr != "" {
		if y, _ := strconv.Atoi(yearFromStr); true {
			query += " AND n.[year] >= ?"
			args = append(args, y)
		}
	}
	if yearToStr != "" {
		if y, _ := strconv.Atoi(yearToStr); true {
			query += " AND n.[year] <= ?"
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
			query += " AND n.[month] IN (" + strings.Join(placeholders, ",") + ")"
		}
	}

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
	appendFilter("n.segment", segments)
	appendFilter("n.channel", channels)

	query += " GROUP BY n.[year], n.[month], n.metric_type, n.un_rub, n.segment, n.channel ORDER BY n.[year] DESC, n.[month] ASC, n.metric_type"

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
