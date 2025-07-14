package db

import (
	"context"
	"database/sql"
	"log"

	"github.com/icrowley/fake"

	"godsendjoseph.dev/sandbox-api/internal/models"
	"godsendjoseph.dev/sandbox-api/internal/store"
)

func Seed(store store.Storage, db *sql.DB) {
	ctx := context.Background()

	// Create users and store them with their DB IDs
	var createdUsers []models.User
	users := generateUsers(50)
	tx, _ := db.BeginTx(ctx, nil)
	for _, user := range users {
		newUser := user // Create a copy to avoid pointer issues
		if err := store.Users.Create(ctx, tx, &newUser); err != nil {
			_ = tx.Rollback()
			log.Printf("error creating user: %v", err)
			return
		}
		createdUsers = append(createdUsers, newUser)
	}

	if err := tx.Commit(); err != nil {
		log.Printf("error committing transaction: %v", err)
		return
	}

	log.Printf("Created %d users", len(createdUsers))

	log.Println("seeding complete")
}

func generateUsers(num int64) []models.User {
	users := make([]models.User, num)

	for i := 0; i < int(num); i++ {
		var pwd models.PasswordHash
		err := pwd.Set("password")

		if err != nil {
			// Handle the error appropriately
			panic(err)
		}
		users[i] = models.User{
			FirstName: fake.FirstName(),
			LastName:  fake.LastName(),
			Username:  fake.UserName(),
			Email:     fake.EmailAddress(),
			IsActive:  true,
			Role: models.Role{
				Name: "user",
			},
			Password: pwd,
		}
	}

	return users
}
