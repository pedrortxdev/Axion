package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func EnsureDBSetup() {
	pgPassword := os.Getenv("PGPASSWORD")
	connStrs := []string{
		"user=postgres host=/var/run/postgresql sslmode=disable",
		"user=postgres host=localhost sslmode=disable",
	}
	if pgPassword != "" {
		connStrs = append(connStrs, fmt.Sprintf("user=postgres password=%s host=localhost sslmode=disable", pgPassword))
	}

	var db *sql.DB
	var err error

	for _, connStr := range connStrs {
		db, err = sql.Open("postgres", connStr)
		if err == nil {
			err = db.Ping()
			if err == nil {
				break
			}
		}
	}

	if err != nil {
		log.Fatalf("Could not connect to PostgreSQL as superuser for bootstrap: %v", err)
	}
	defer db.Close()

	// Check 1: Ensure 'axion' user exists
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = 'axion')").Scan(&exists)
	if err != nil {
		log.Fatalf("Failed to check for user 'axion': %v", err)
	}

	if !exists {
		log.Println("User 'axion' does not exist. Creating...")
		_, err = db.Exec("CREATE USER axion WITH PASSWORD 'axion_password' SUPERUSER;")
		if err != nil {
			log.Fatalf("Failed to create user 'axion': %v", err)
		}
		log.Println("User 'axion' created successfully.")
	}

	// Check 2: Ensure 'axion_db' database exists
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = 'axion_db')").Scan(&exists)
	if err != nil {
		log.Fatalf("Failed to check for database 'axion_db': %v", err)
	}

	if !exists {
		log.Println("Database 'axion_db' does not exist. Creating...")
		_, err = db.Exec("CREATE DATABASE axion_db OWNER axion;")
		if err != nil {
			log.Fatalf("Failed to create database 'axion_db': %v", err)
		}
		log.Println("Database 'axion_db' created successfully.")
	}

	fmt.Println("Database setup verified.")
}
