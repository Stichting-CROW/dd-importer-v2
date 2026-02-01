package main

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
)

func initPostgresDB() *pgx.Conn {
	connStr := os.Getenv("PGURL")
	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Connected to PostgreSQL database")
	return conn
}
