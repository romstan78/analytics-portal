package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"backend/config"
	"backend/models"
)

var ErrDictionaryNotFound = errors.New("запись справочника не найдена")

func GetDictionaries() (models.DictionaryData, error) {
	result := models.DictionaryData{
		SKUs: []models.SKUReference{}, Networks: []models.NetworkReference{},
		KAMNetworks: []models.KAMNetworkReference{}, Mechanics: []models.MechanicReference{},
	}

	skuRows, err := config.DB.Query(`SELECT id, sku, ISNULL(brand,''), ISNULL(brand_as,''), CONVERT(NVARCHAR, created_at, 23) FROM dbo.tbl_SKUMapping ORDER BY sku`)
	if err != nil {
		return result, err
	}
	defer skuRows.Close()
	for skuRows.Next() {
		var row models.SKUReference
		if err := skuRows.Scan(&row.ID, &row.SKU, &row.Brand, &row.BrandAS, &row.CreatedAt); err != nil {
			return result, err
		}
		result.SKUs = append(result.SKUs, row)
	}
	if err := skuRows.Err(); err != nil {
		return result, err
	}

	networkRows, err := config.DB.Query(`SELECT id, network_name, ISNULL(kam,''), ISNULL(network_type,''), ISNULL(top20_segment,''), ISNULL(key_region,'') FROM dbo.tbl_NetworkGeoMapping ORDER BY network_name`)
	if err != nil {
		return result, err
	}
	defer networkRows.Close()
	for networkRows.Next() {
		var row models.NetworkReference
		if err := networkRows.Scan(&row.ID, &row.NetworkName, &row.KAM, &row.NetworkType, &row.Top20Segment, &row.KeyRegion); err != nil {
			return result, err
		}
		result.Networks = append(result.Networks, row)
	}
	if err := networkRows.Err(); err != nil {
		return result, err
	}

	kamRows, err := config.DB.Query(`SELECT id, kam, network_name, CONVERT(NVARCHAR, valid_from, 23), CONVERT(NVARCHAR, created_at, 23) FROM dbo.tbl_KAMNetworkMapping ORDER BY kam, network_name, valid_from DESC`)
	if err != nil {
		return result, err
	}
	defer kamRows.Close()
	for kamRows.Next() {
		var row models.KAMNetworkReference
		if err := kamRows.Scan(&row.ID, &row.KAM, &row.NetworkName, &row.ValidFrom, &row.CreatedAt); err != nil {
			return result, err
		}
		result.KAMNetworks = append(result.KAMNetworks, row)
	}
	if err := kamRows.Err(); err != nil {
		return result, err
	}

	mechanicRows, err := config.DB.Query(`SELECT id, mechanics, channel, ISNULL(short_code,''), CONVERT(NVARCHAR, created_at, 23) FROM dbo.tbl_MechanicsChannelMapping ORDER BY mechanics`)
	if err != nil {
		return result, err
	}
	defer mechanicRows.Close()
	for mechanicRows.Next() {
		var row models.MechanicReference
		if err := mechanicRows.Scan(&row.ID, &row.Mechanics, &row.Channel, &row.ShortCode, &row.CreatedAt); err != nil {
			return result, err
		}
		result.Mechanics = append(result.Mechanics, row)
	}
	return result, mechanicRows.Err()
}

func auditDictionaryTx(tx *sql.Tx, entityType string, entityID int, username, action string, oldValue, newValue any) error {
	changed, err := json.Marshal(map[string]any{"old": oldValue, "new": newValue})
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO dbo.tbl_AuditLog (entity_type, entity_id, user_name, action_type, changed_fields) VALUES (?, ?, ?, ?, ?)`, entityType, entityID, username, action, string(changed))
	return err
}

func CreateSKUReference(input models.SKUReference, username string) (models.SKUReference, error) {
	tx, err := config.DB.Begin()
	if err != nil {
		return input, err
	}
	defer tx.Rollback()
	err = tx.QueryRow(`INSERT INTO dbo.tbl_SKUMapping(sku,brand,brand_as) OUTPUT INSERTED.id, CONVERT(NVARCHAR, INSERTED.created_at, 23) VALUES (?,?,?)`, input.SKU, nullable(input.Brand), nullable(input.BrandAS)).Scan(&input.ID, &input.CreatedAt)
	if err != nil {
		return input, err
	}
	if err = auditDictionaryTx(tx, "dictionary_sku", input.ID, username, "CREATE", nil, input); err != nil {
		return input, err
	}
	return input, tx.Commit()
}

func UpdateSKUReference(id int, input models.SKUReference, username string) (models.SKUReference, error) {
	tx, err := config.DB.Begin()
	if err != nil {
		return input, err
	}
	defer tx.Rollback()
	var old models.SKUReference
	err = tx.QueryRow(`SELECT id,sku,ISNULL(brand,''),ISNULL(brand_as,''),CONVERT(NVARCHAR,created_at,23) FROM dbo.tbl_SKUMapping WITH (UPDLOCK) WHERE id=?`, id).Scan(&old.ID, &old.SKU, &old.Brand, &old.BrandAS, &old.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return input, ErrDictionaryNotFound
	}
	if err != nil {
		return input, err
	}
	input.ID, input.SKU, input.CreatedAt = id, old.SKU, old.CreatedAt
	if _, err = tx.Exec(`UPDATE dbo.tbl_SKUMapping SET brand=?,brand_as=? WHERE id=?`, nullable(input.Brand), nullable(input.BrandAS), id); err != nil {
		return input, err
	}
	if err = auditDictionaryTx(tx, "dictionary_sku", id, username, "UPDATE", old, input); err != nil {
		return input, err
	}
	return input, tx.Commit()
}

func CreateNetworkReference(input models.NetworkReference, username string) (models.NetworkReference, error) {
	tx, err := config.DB.Begin()
	if err != nil {
		return input, err
	}
	defer tx.Rollback()
	err = tx.QueryRow(`INSERT INTO dbo.tbl_NetworkGeoMapping(network_name,kam,network_type,top20_segment,key_region) OUTPUT INSERTED.id VALUES (?,?,?,?,?)`, input.NetworkName, nullable(input.KAM), nullable(input.NetworkType), nullable(input.Top20Segment), nullable(input.KeyRegion)).Scan(&input.ID)
	if err != nil {
		return input, err
	}
	if err = auditDictionaryTx(tx, "dictionary_network", input.ID, username, "CREATE", nil, input); err != nil {
		return input, err
	}
	return input, tx.Commit()
}

func UpdateNetworkReference(id int, input models.NetworkReference, username string) (models.NetworkReference, error) {
	tx, err := config.DB.Begin()
	if err != nil {
		return input, err
	}
	defer tx.Rollback()
	var old models.NetworkReference
	err = tx.QueryRow(`SELECT id,network_name,ISNULL(kam,''),ISNULL(network_type,''),ISNULL(top20_segment,''),ISNULL(key_region,'') FROM dbo.tbl_NetworkGeoMapping WITH (UPDLOCK) WHERE id=?`, id).Scan(&old.ID, &old.NetworkName, &old.KAM, &old.NetworkType, &old.Top20Segment, &old.KeyRegion)
	if errors.Is(err, sql.ErrNoRows) {
		return input, ErrDictionaryNotFound
	}
	if err != nil {
		return input, err
	}
	input.ID, input.NetworkName = id, old.NetworkName
	_, err = tx.Exec(`UPDATE dbo.tbl_NetworkGeoMapping SET kam=?,network_type=?,top20_segment=?,key_region=? WHERE id=?`, nullable(input.KAM), nullable(input.NetworkType), nullable(input.Top20Segment), nullable(input.KeyRegion), id)
	if err != nil {
		return input, err
	}
	if err = auditDictionaryTx(tx, "dictionary_network", id, username, "UPDATE", old, input); err != nil {
		return input, err
	}
	return input, tx.Commit()
}

func CreateKAMNetworkReference(input models.KAMNetworkReference, username string) (models.KAMNetworkReference, error) {
	tx, err := config.DB.Begin()
	if err != nil {
		return input, err
	}
	defer tx.Rollback()
	err = tx.QueryRow(`INSERT INTO dbo.tbl_KAMNetworkMapping(kam,network_name,valid_from) OUTPUT INSERTED.id, CONVERT(NVARCHAR,INSERTED.created_at,23) VALUES (?,?,?)`, input.KAM, input.NetworkName, input.ValidFrom).Scan(&input.ID, &input.CreatedAt)
	if err != nil {
		return input, err
	}
	if err = auditDictionaryTx(tx, "dictionary_kam_network", input.ID, username, "CREATE", nil, input); err != nil {
		return input, err
	}
	return input, tx.Commit()
}

func UpdateKAMNetworkReference(id int, input models.KAMNetworkReference, username string) (models.KAMNetworkReference, error) {
	tx, err := config.DB.Begin()
	if err != nil {
		return input, err
	}
	defer tx.Rollback()
	var old models.KAMNetworkReference
	err = tx.QueryRow(`SELECT id,kam,network_name,CONVERT(NVARCHAR,valid_from,23),CONVERT(NVARCHAR,created_at,23) FROM dbo.tbl_KAMNetworkMapping WITH (UPDLOCK) WHERE id=?`, id).Scan(&old.ID, &old.KAM, &old.NetworkName, &old.ValidFrom, &old.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return input, ErrDictionaryNotFound
	}
	if err != nil {
		return input, err
	}
	input.ID, input.CreatedAt = id, old.CreatedAt
	_, err = tx.Exec(`UPDATE dbo.tbl_KAMNetworkMapping SET kam=?,network_name=?,valid_from=? WHERE id=?`, input.KAM, input.NetworkName, input.ValidFrom, id)
	if err != nil {
		return input, err
	}
	if err = auditDictionaryTx(tx, "dictionary_kam_network", id, username, "UPDATE", old, input); err != nil {
		return input, err
	}
	return input, tx.Commit()
}

func CreateMechanicReference(input models.MechanicReference, username string) (models.MechanicReference, error) {
	tx, err := config.DB.Begin()
	if err != nil {
		return input, err
	}
	defer tx.Rollback()
	err = tx.QueryRow(`INSERT INTO dbo.tbl_MechanicsChannelMapping(mechanics,channel,short_code) OUTPUT INSERTED.id, CONVERT(NVARCHAR,INSERTED.created_at,23) VALUES (?,?,?)`, input.Mechanics, input.Channel, nullable(input.ShortCode)).Scan(&input.ID, &input.CreatedAt)
	if err != nil {
		return input, err
	}
	if err = auditDictionaryTx(tx, "dictionary_mechanic", input.ID, username, "CREATE", nil, input); err != nil {
		return input, err
	}
	return input, tx.Commit()
}

func UpdateMechanicReference(id int, input models.MechanicReference, username string) (models.MechanicReference, error) {
	tx, err := config.DB.Begin()
	if err != nil {
		return input, err
	}
	defer tx.Rollback()
	var old models.MechanicReference
	err = tx.QueryRow(`SELECT id,mechanics,channel,ISNULL(short_code,''),CONVERT(NVARCHAR,created_at,23) FROM dbo.tbl_MechanicsChannelMapping WITH (UPDLOCK) WHERE id=?`, id).Scan(&old.ID, &old.Mechanics, &old.Channel, &old.ShortCode, &old.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return input, ErrDictionaryNotFound
	}
	if err != nil {
		return input, err
	}
	input.ID, input.Mechanics, input.CreatedAt = id, old.Mechanics, old.CreatedAt
	_, err = tx.Exec(`UPDATE dbo.tbl_MechanicsChannelMapping SET channel=?,short_code=? WHERE id=?`, input.Channel, nullable(input.ShortCode), id)
	if err != nil {
		return input, err
	}
	if err = auditDictionaryTx(tx, "dictionary_mechanic", id, username, "UPDATE", old, input); err != nil {
		return input, err
	}
	return input, tx.Commit()
}

func nullable(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func IsDuplicateDictionaryError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(fmt.Sprint(err))
	return strings.Contains(message, "duplicate") || strings.Contains(message, "unique") || strings.Contains(message, "2601") || strings.Contains(message, "2627")
}
