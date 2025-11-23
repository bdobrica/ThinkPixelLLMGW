package main

import (
	"log"
	"net/http"

	"github.com/bdobrica/ThinkPixelLLMGW/llm-gateway/internal/config"
	"github.com/bdobrica/ThinkPixelLLMGW/llm-gateway/internal/httpapi"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	mux, err := httpapi.NewRouter(cfg)
	if err != nil {
		log.Fatalf("failed to build router: %v", err)
	}

	addr := ":" + cfg.HTTPPort
	log.Printf("llm-gateway listening on %s", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
