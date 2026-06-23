package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/kubixhq/kubix-migration-tool/internal/detector"
)

type Handler struct {
	db *sql.DB
}

func New(db *sql.DB) *Handler {
	return &Handler{db: db}
}

// GET /api/migration/detect
// Scans the DB and returns every migration tool found.
func (h *Handler) Detect(w http.ResponseWriter, r *http.Request) {
	results := detector.Detect(h.db)
	writeJSON(w, http.StatusOK, results)
}

// GET /api/migration/tool
// Returns the resolved tool: manual preference if set, otherwise auto-detect.
func (h *Handler) GetTool(w http.ResponseWriter, r *http.Request) {
	type response struct {
		Tool       string `json:"tool"`
		Confidence string `json:"confidence"`
		Evidence   string `json:"evidence"`
		UpdatedAt  string `json:"updatedAt,omitempty"`
	}

	// check manual preference
	var pref, updatedAt string
	err := h.db.QueryRowContext(r.Context(), `
		SELECT value, updated_at::text FROM kubix_migration_config WHERE key = 'tool_preference'
	`).Scan(&pref, &updatedAt)

	if err == nil && pref != "" && pref != "auto" {
		writeJSON(w, http.StatusOK, response{
			Tool:       pref,
			Confidence: "manual",
			Evidence:   "Set manually by user",
			UpdatedAt:  updatedAt,
		})
		return
	}

	// fall back to auto-detect
	results := detector.Detect(h.db)
	primary := detector.Primary(results)
	writeJSON(w, http.StatusOK, response{
		Tool:       string(primary.Tool),
		Confidence: "auto",
		Evidence:   primary.Evidence,
	})
}

// PUT /api/migration/tool  { "tool": "flyway" | "liquibase" | "prisma" | "auto" }
// Stores the user's tool preference.
func (h *Handler) SetTool(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Tool string `json:"tool"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	allowed := map[string]bool{"flyway": true, "liquibase": true, "prisma": true, "auto": true}
	if !allowed[req.Tool] {
		writeError(w, http.StatusBadRequest, "tool must be one of: flyway, liquibase, prisma, auto")
		return
	}

	_, err := h.db.ExecContext(r.Context(), `
		INSERT INTO kubix_migration_config (key, value, updated_at)
		VALUES ('tool_preference', $1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at
	`, req.Tool, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save preference")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"tool": req.Tool, "status": "saved"})
}

// DELETE /api/migration/tool
// Clears the manual preference (reverts to auto-detect).
func (h *Handler) ResetTool(w http.ResponseWriter, r *http.Request) {
	_, _ = h.db.ExecContext(r.Context(), `
		DELETE FROM kubix_migration_config WHERE key = 'tool_preference'
	`)
	writeJSON(w, http.StatusOK, map[string]string{"status": "reset to auto"})
}

// helpers

type errResp struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errResp{Error: msg})
}
