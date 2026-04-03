package main

import (
	"log"

	"github.com/wuyaocheng/bktrader/internal/app"
	"github.com/wuyaocheng/bktrader/internal/config"
)

func main() {
	cfg := config.Load()
	server, err := app.NewServer(cfg)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("platform-api listening on %s", cfg.HTTPAddr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
