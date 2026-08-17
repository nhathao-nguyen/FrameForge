package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/persistence"
)

func main() {
	dsn := flag.String("dsn", os.Getenv("NH_MEDIA_DATABASE_URL"), "PostgreSQL DSN")
	directory := flag.String("dir", "services/api/migrations", "migration directory")
	appRole := flag.String("app-role", os.Getenv("POSTGRES_APP_USER"), "optional Product API database role")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := persistence.OpenPostgres(ctx, *dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	runner, err := persistence.NewMigrationRunner(db, *directory)
	if err != nil {
		log.Fatal(err)
	}
	if err := runner.Apply(ctx); err != nil {
		log.Fatal(err)
	}
	if *appRole != "" {
		if err := runner.GrantAppRole(ctx, *appRole); err != nil {
			log.Fatal(err)
		}
	}
	log.Printf("NH-Media migrations applied successfully")
}
