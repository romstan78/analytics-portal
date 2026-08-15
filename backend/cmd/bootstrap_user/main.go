package main

import (
	"backend/config"
	"database/sql"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{3,100}$`)

func main() {
	_ = godotenv.Load()
	username := strings.TrimSpace(os.Getenv("BOOTSTRAP_USERNAME"))
	password := os.Getenv("BOOTSTRAP_PASSWORD")
	role := strings.TrimSpace(os.Getenv("BOOTSTRAP_ROLE"))

	if !usernamePattern.MatchString(username) {
		log.Fatal("BOOTSTRAP_USERNAME: 3-100 символов; разрешены латинские буквы, цифры, ., _ и -")
	}
	if len(password) < 12 {
		log.Fatal("BOOTSTRAP_PASSWORD должен содержать минимум 12 символов")
	}
	switch role {
	case "admin", "agreement1", "agreement2":
	default:
		log.Fatal("BOOTSTRAP_ROLE должен быть admin, agreement1 или agreement2")
	}

	if err := config.Init(); err != nil {
		log.Fatalf("Ошибка инициализации БД: %v", err)
	}
	defer config.DB.Close()

	var existingID int
	err := config.DB.QueryRow("SELECT id FROM dbo.tbl_Users WHERE username = ?", username).Scan(&existingID)
	if err == nil {
		log.Fatalf("Пользователь %q уже существует; его пароль и роль не изменены", username)
	}
	if err != sql.ErrNoRows {
		log.Fatalf("Ошибка проверки пользователя: %v", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		log.Fatalf("Ошибка хеширования пароля: %v", err)
	}
	if _, err := config.DB.Exec(
		"INSERT INTO dbo.tbl_Users(username, password_hash, role) VALUES (?, ?, ?)",
		username, string(hash), role,
	); err != nil {
		log.Fatalf("Ошибка создания пользователя: %v", err)
	}

	fmt.Printf("Пользователь %q создан с ролью %q.\n", username, role)
}
