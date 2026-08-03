//go:build ignore
// +build ignore

// Утилита для генерации bcrypt-хеша из пароля.
// Использование:
//   go run cmd/hash_password.go ваш_пароль
//
// Выводит bcrypt-хеш (cost=10), который можно вставить в tbl_Users.password_hash.

package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Использование: go run cmd/hash_password.go <пароль>")
		os.Exit(1)
	}

	password := os.Args[1]
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(hash))
}
