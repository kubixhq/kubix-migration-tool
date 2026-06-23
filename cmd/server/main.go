package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/kubixhq/kubix-migration-tool/internal/config"
	"github.com/kubixhq/kubix-migration-tool/internal/db"
	"github.com/kubixhq/kubix-migration-tool/internal/handler"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	database, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("cannot connect to database: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	h := handler.New(database)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /api/migration/detect", h.Detect)
	mux.HandleFunc("GET /api/migration/tool", h.GetTool)
	mux.HandleFunc("PUT /api/migration/tool", h.SetTool)
	mux.HandleFunc("DELETE /api/migration/tool", h.ResetTool)

	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	log.Printf("kubix-migration-tool listening on %s", addr)
	if err := http.ListenAndServe(addr, cors(mux)); err != nil {
		log.Fatal(err)
	}
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
