// Command migrate applies or rolls back database migrations.
package main

import (
	"log"
	"os"

	"github.com/isAdamBailey/face-value/backend/internal/db"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}

	direction := "up"
	if len(os.Args) > 1 {
		direction = os.Args[1]
	}

	switch direction {
	case "up":
		if err := db.Migrate(dsn); err != nil {
			log.Fatalf("migrate up: %v", err)
		}
		log.Println("migrations applied")
	case "down":
		if err := db.MigrateDown(dsn); err != nil {
			log.Fatalf("migrate down: %v", err)
		}
		log.Println("migrations rolled back")
	default:
		log.Fatalf("unknown migrate direction %q (expected up or down)", direction)
	}
}
