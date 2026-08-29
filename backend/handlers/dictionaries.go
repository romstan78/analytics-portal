package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend/config"
	"backend/models"
	"backend/repository"

	"github.com/gin-gonic/gin"
)

func GetDictionaries(c *gin.Context) {
	data, err := repository.GetDictionaries()
	if err != nil {
		config.Logger.Error("get_dictionaries_failed", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить справочники"})
		return
	}
	c.JSON(http.StatusOK, data)
}

func dictionaryID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный идентификатор записи"})
		return 0, false
	}
	return id, true
}

func requiredDictionaryValue(value, label string, max int) (string, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", label + " — обязательное поле"
	}
	if len([]rune(value)) > max {
		return "", label + ": превышена допустимая длина"
	}
	return value, ""
}

func optionalDictionaryValue(value, label string, max int) (string, string) {
	value = strings.TrimSpace(value)
	if len([]rune(value)) > max {
		return "", label + ": превышена допустимая длина"
	}
	return value, ""
}

func dictionaryWriteError(c *gin.Context, err error) {
	if errors.Is(err, repository.ErrDictionaryNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if repository.IsDuplicateDictionaryError(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "Такая запись уже существует"})
		return
	}
	config.Logger.Error("dictionary_write_failed", "error", err.Error())
	c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить запись справочника"})
}

func cleanSKU(input *models.SKUReference) string {
	var message string
	if input.SKU, message = requiredDictionaryValue(input.SKU, "SKU", 255); message != "" {
		return message
	}
	if input.Brand, message = optionalDictionaryValue(input.Brand, "Бренд", 255); message != "" {
		return message
	}
	if input.BrandAS, message = optionalDictionaryValue(input.BrandAS, "Бренд АС", 255); message != "" {
		return message
	}
	return ""
}

func CreateSKUReference(c *gin.Context) {
	var input models.SKUReference
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные"})
		return
	}
	if message := cleanSKU(&input); message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": message})
		return
	}
	row, err := repository.CreateSKUReference(input, c.GetString("username"))
	if err != nil {
		dictionaryWriteError(c, err)
		return
	}
	c.JSON(http.StatusCreated, row)
}

func UpdateSKUReference(c *gin.Context) {
	id, ok := dictionaryID(c)
	if !ok {
		return
	}
	var input models.SKUReference
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные"})
		return
	}
	if message := cleanSKU(&input); message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": message})
		return
	}
	row, err := repository.UpdateSKUReference(id, input, c.GetString("username"))
	if err != nil {
		dictionaryWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, row)
}

func cleanNetwork(input *models.NetworkReference) string {
	var message string
	if input.NetworkName, message = requiredDictionaryValue(input.NetworkName, "Название сети", 256); message != "" {
		return message
	}
	if input.KAM, message = optionalDictionaryValue(input.KAM, "КАМ", 128); message != "" {
		return message
	}
	if input.NetworkType, message = optionalDictionaryValue(input.NetworkType, "Тип сети", 128); message != "" {
		return message
	}
	if input.Top20Segment, message = optionalDictionaryValue(input.Top20Segment, "Сегмент", 128); message != "" {
		return message
	}
	if input.KeyRegion, message = optionalDictionaryValue(input.KeyRegion, "Ключевой регион", 128); message != "" {
		return message
	}
	return ""
}

func CreateNetworkReference(c *gin.Context) {
	var input models.NetworkReference
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные"})
		return
	}
	if message := cleanNetwork(&input); message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": message})
		return
	}
	row, err := repository.CreateNetworkReference(input, c.GetString("username"))
	if err != nil {
		dictionaryWriteError(c, err)
		return
	}
	c.JSON(http.StatusCreated, row)
}

func UpdateNetworkReference(c *gin.Context) {
	id, ok := dictionaryID(c)
	if !ok {
		return
	}
	var input models.NetworkReference
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные"})
		return
	}
	if message := cleanNetwork(&input); message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": message})
		return
	}
	row, err := repository.UpdateNetworkReference(id, input, c.GetString("username"))
	if err != nil {
		dictionaryWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, row)
}

func cleanKAMNetwork(input *models.KAMNetworkReference) string {
	var message string
	if input.KAM, message = requiredDictionaryValue(input.KAM, "КАМ", 255); message != "" {
		return message
	}
	if input.NetworkName, message = requiredDictionaryValue(input.NetworkName, "Название сети", 255); message != "" {
		return message
	}
	input.ValidFrom = strings.TrimSpace(input.ValidFrom)
	if _, err := time.Parse("2006-01-02", input.ValidFrom); err != nil {
		return "Дата начала должна быть в формате ГГГГ-ММ-ДД"
	}
	return ""
}

func CreateKAMNetworkReference(c *gin.Context) {
	var input models.KAMNetworkReference
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные"})
		return
	}
	if message := cleanKAMNetwork(&input); message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": message})
		return
	}
	row, err := repository.CreateKAMNetworkReference(input, c.GetString("username"))
	if err != nil {
		dictionaryWriteError(c, err)
		return
	}
	c.JSON(http.StatusCreated, row)
}

func UpdateKAMNetworkReference(c *gin.Context) {
	id, ok := dictionaryID(c)
	if !ok {
		return
	}
	var input models.KAMNetworkReference
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные"})
		return
	}
	if message := cleanKAMNetwork(&input); message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": message})
		return
	}
	row, err := repository.UpdateKAMNetworkReference(id, input, c.GetString("username"))
	if err != nil {
		dictionaryWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, row)
}

func cleanMechanic(input *models.MechanicReference) string {
	var message string
	if input.Mechanics, message = requiredDictionaryValue(input.Mechanics, "Механика", 255); message != "" {
		return message
	}
	if input.Channel, message = requiredDictionaryValue(input.Channel, "Канал", 100); message != "" {
		return message
	}
	if input.ShortCode, message = optionalDictionaryValue(input.ShortCode, "Короткий код", 12); message != "" {
		return message
	}
	return ""
}

func CreateMechanicReference(c *gin.Context) {
	var input models.MechanicReference
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные"})
		return
	}
	if message := cleanMechanic(&input); message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": message})
		return
	}
	row, err := repository.CreateMechanicReference(input, c.GetString("username"))
	if err != nil {
		dictionaryWriteError(c, err)
		return
	}
	c.JSON(http.StatusCreated, row)
}

func UpdateMechanicReference(c *gin.Context) {
	id, ok := dictionaryID(c)
	if !ok {
		return
	}
	var input models.MechanicReference
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные"})
		return
	}
	if message := cleanMechanic(&input); message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": message})
		return
	}
	row, err := repository.UpdateMechanicReference(id, input, c.GetString("username"))
	if err != nil {
		dictionaryWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, row)
}
