package detector

import (
	"context"
	"database/sql"
	"time"
)

type Tool string

const (
	ToolFlyway    Tool = "flyway"
	ToolLiquibase Tool = "liquibase"
	ToolPrisma    Tool = "prisma"
	ToolNone      Tool = "none"
)

type DetectionResult struct {
	Tool       Tool   `json:"tool"`
	Confidence string `json:"confidence"` // "auto" | "manual"
	Evidence   string `json:"evidence"`   // which table was found
	Found      bool   `json:"found"`
}

var probes = []struct {
	tool  Tool
	table string
	label string
}{
	{ToolFlyway, "flyway_schema_history", "flyway_schema_history table found"},
	{ToolLiquibase, "databasechangelog", "databasechangelog table found"},
	{ToolPrisma, "_prisma_migrations", "_prisma_migrations table found"},
}

// Detect scans the connected database for known migration tool tables.
// Returns all tools that were detected.
func Detect(db *sql.DB) []DetectionResult {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var results []DetectionResult
	for _, p := range probes {
		var exists bool
		_ = db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
			)
		`, p.table).Scan(&exists)

		if exists {
			results = append(results, DetectionResult{
				Tool:       p.tool,
				Confidence: "auto",
				Evidence:   p.label,
				Found:      true,
			})
		}
	}

	if len(results) == 0 {
		results = append(results, DetectionResult{
			Tool:       ToolNone,
			Confidence: "auto",
			Evidence:   "No known migration tables found",
			Found:      false,
		})
	}
	return results
}

// Primary returns the single best-guess tool (first detected or none).
func Primary(results []DetectionResult) DetectionResult {
	for _, r := range results {
		if r.Found {
			return r
		}
	}
	return results[0]
}
