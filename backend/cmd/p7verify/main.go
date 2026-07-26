package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/pkg/pagination"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	maxPaginationPages = 100
	defaultPageSize    = 50
)

func main() {
	mode := flag.String("mode", "", "pagination|query-plan|nplusone|provider-concurrency|provider-adaptive|permission-invalidation")
	dryRun := flag.Bool("dry-run", false, "validate guards only without touching the database")
	flag.Parse()

	started := time.Now().UTC()
	m := strings.ToLower(strings.TrimSpace(*mode))
	if m == "" {
		failReport("invalid_mode", "missing --mode")
		os.Exit(2)
	}

	if *dryRun {
		if err := validateGuardsFromEnv(); err != nil {
			failReport("guard_rejected", err.Error())
			os.Exit(1)
		}
		writeJSON(map[string]any{
			"phase":       phase,
			"status":      "dry_run_passed",
			"mode":        m,
			"generatedAt": time.Now().UTC().Format(time.RFC3339),
			"guards":      guardList(),
			"durationMs":  time.Since(started).Milliseconds(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	var (
		rep any
		err error
	)
	switch m {
	case "pagination":
		rep, err = runPagination(ctx)
	case "query-plan":
		rep, err = runQueryPlan(ctx)
	case "nplusone":
		rep, err = runNPlusOne(ctx)
	case "provider-concurrency":
		rep, err = runProviderConcurrency(ctx)
	case "provider-adaptive":
		rep, err = runProviderAdaptive(ctx)
	case "permission-invalidation":
		rep, err = runPermissionInvalidation(ctx)
	default:
		failReport("invalid_mode", fmt.Sprintf("unknown mode %q", m))
		os.Exit(2)
	}
	if err != nil {
		if strings.Contains(err.Error(), "guard") || strings.Contains(err.Error(), "forbidden") {
			failReport("guard_rejected", err.Error())
			os.Exit(1)
		}
		failReport("failed", err.Error())
		os.Exit(1)
	}
	writeJSON(rep)
	if r, ok := rep.(paginationReport); ok {
		if r.Status == "failed" {
			os.Exit(1)
		}
		return
	}
	if r, ok := rep.(queryPlanReport); ok && r.Status == "failed" {
		os.Exit(1)
	}
	if r, ok := rep.(nPlusOneReport); ok && r.Status == "failed" {
		os.Exit(1)
	}
	if r, ok := rep.(providerConcurrencyReport); ok && r.Status == "failed" {
		os.Exit(1)
	}
	if r, ok := rep.(providerAdaptiveReport); ok && r.Status == "failed" {
		os.Exit(1)
	}
	if r, ok := rep.(permissionReport); ok && r.Status == "failed" {
		os.Exit(1)
	}
}

func openVerifiedDB(ctx context.Context) (*env, error) {
	e, err := openEnv(ctx)
	if err != nil {
		return nil, err
	}
	e.db = e.db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	return e, nil
}

func tamperCursor(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw + "tampered"
	}
	return raw[:len(raw)-1] + "X"
}

func guardRejected(err error, code string) bool {
	if err == nil {
		return false
	}
	return pagination.ErrorCode(err) == code || isGuardError(err, code)
}

func closeEnv(e *env) {
	if e != nil && e.db != nil {
		_ = database.Close(e.db)
	}
}
