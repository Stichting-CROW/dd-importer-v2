package main

import (
	"deelfietsdashboard-importer/monitor"
	"fmt"
	"log"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	log.Println("Starting Feed Monitor Service")

	// Initialize database connection
	db, err := initDatabase()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Database connection established")

	// Initialize Telegram notifier
	notifier, err := monitor.NewTelegramNotifier()
	if err != nil {
		log.Fatalf("Failed to initialize Telegram notifier: %v", err)
	}
	log.Println("Telegram notifier initialized")

	// Send startup notification
	err = notifier.SendAlert(fmt.Sprintf(
		"📊 <b>Feed Monitor Started</b>\n\nMonitoring %s@%s",
		os.Getenv("PGDATABASE"),
		os.Getenv("PGHOST"),
	))
	if err != nil {
		log.Printf("Warning: Failed to send startup notification: %v", err)
	}

	// Initialize and start feed monitor
	feedMonitor := monitor.NewFeedMonitor(db, notifier)
	log.Println("Starting feed monitoring loop (checking every minute)...")

	// Start monitoring (this runs indefinitely)
	feedMonitor.MonitorFeeds()
}

// initDatabase initializes the PostgreSQL database connection
func initDatabase() (*sqlx.DB, error) {
	connStr := fmt.Sprintf(
		"dbname=%s user=%s host=%s password=%s sslmode=disable",
		getEnvOrDefault("PGDATABASE", "deelfietsdashboard"),
		getEnvOrDefault("PGUSER", "deelfietsdashboard"),
		getEnvOrDefault("PGHOST", "localhost"),
		getEnvOrDefault("PGPASSWORD", ""),
	)

	db, err := sqlx.Connect("postgres", connStr+" binary_parameters=yes")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}

// getEnvOrDefault returns the environment variable value or a default if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
