package main

import (
	"fmt"
	"log"

	"godsendjoseph.dev/sandbox-api/internal/db"
	"godsendjoseph.dev/sandbox-api/internal/env"
	"godsendjoseph.dev/sandbox-api/internal/store"
)

func main() {
	conn, err := db.New(
		fmt.Sprintf("%s:%s", env.GetString("DB_HOST", "127.0.0.1"), env.GetString("DB_PORT", "3306")),
		env.GetString("DB_USER", "root"),
		env.GetString("DB_PASSWORD", "password"),
		env.GetString("DB_NAME", "social_api_db"),
		env.GetInt("DB_MAX_OPEN_CONNS", 25),
		env.GetInt("DB_MAX_IDLE_CONNS", 25),
		env.GetString("DB_MAX_IDLE_TIME", "15m"),
	)

	if err != nil {
		log.Panic(err)
	}

	defer conn.Close()

	store := store.NewStorage(conn)
	db.Seed(store, conn)
}
