package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/PauloHFS/elenchus/internal/db"
	"golang.org/x/crypto/bcrypt"
)

func RunCreateUser() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: create-user <email> <password>")
		os.Exit(1)
	}
	email := os.Args[2]
	password := os.Args[3]

	dbConn, err := initDB()
	if err != nil {
		panic(err)
	}
	defer dbConn.Close()
	queries := db.New(dbConn)

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}

	_, err = queries.CreateUser(context.Background(), db.CreateUserParams{
		TenantID:     "default",
		Email:        email,
		PasswordHash: string(hash),
		RoleID:       "user",
	})
	if err != nil {
		fmt.Printf("failed to create user: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("User %s created successfully\n", email)
}
