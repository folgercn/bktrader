package main

import (
	"log"

	"github.com/wuyaocheng/bktrader/internal/config"
	"github.com/wuyaocheng/bktrader/internal/store/postgres"
)

func main() {
	cfg := config.Load()

	if err := postgres.Migrate(cfg.PostgresDSN); err != nil {
		log.Fatal(err)
	}

	log.Printf("database migrations applied successfully")
}
