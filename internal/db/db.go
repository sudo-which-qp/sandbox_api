package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/go-sql-driver/mysql"
)

func New(addr, user, password, dbName string, maxOpenConns, maxIdleConns int, maxIdleTime string) (*sql.DB, error) {
	dbConfig := mysql.Config{
		User:                 user,
		Passwd:               password,
		Addr:                 addr,
		DBName:               dbName,
		Net:                  "tcp",
		AllowNativePasswords: true,
		ParseTime:            true,
	}

	var db *sql.DB
	var err error

	// Retry logic
	for i := 0; i < 10; i++ {
		db, err = sql.Open("mysql", dbConfig.FormatDSN())
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err = db.PingContext(ctx); err == nil {
				duration, err := time.ParseDuration(maxIdleTime)
				if err != nil {
					log.Fatal(err)
					return nil, err
				}

				db.SetMaxOpenConns(maxOpenConns)
				db.SetMaxIdleConns(maxIdleConns)
				db.SetConnMaxIdleTime(duration)

				return db, nil
			}
		}
		log.Printf("Attempt %d: Database not ready, retrying...", i+1)
		time.Sleep(5 * time.Second)
	}
	return nil, fmt.Errorf("could not connect to the database after multiple attempts: %v", err)
}
