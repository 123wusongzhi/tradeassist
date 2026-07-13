package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/modules/performance"
	"gorm.io/datatypes"
)

type report struct {
	Phase       string         `json:"phase"`
	Status      string         `json:"status"`
	RunID       string         `json:"runId"`
	Profile     string         `json:"profile"`
	DryRun      bool           `json:"dryRun"`
	DatasetPlan map[string]int `json:"datasetPlan"`
	RowsPlanned int64          `json:"rowsPlanned"`
	RowsWritten int64          `json:"rowsWritten"`
	StartedAt   string         `json:"startedAt"`
	FinishedAt  string         `json:"finishedAt"`
	Guards      []string       `json:"guards"`
	Issues      []string       `json:"issues"`
}

func main() {
	profile := flag.String("profile", "small", "small|medium|large|stress")
	runID := flag.String("run-id", "", "P7 run id")
	dryRun := flag.Bool("dry-run", true, "only validate guards and print dataset plan")
	flag.Parse()

	start := time.Now().UTC()
	id := strings.TrimSpace(*runID)
	if id == "" {
		id = "p7-" + start.Format("20060102T150405Z")
	}
	plan, rows, err := datasetPlan(*profile)
	if err != nil {
		write(report{Phase: "P7", Status: "invalid_profile", RunID: id, Profile: *profile, DryRun: *dryRun, Issues: []string{err.Error()}})
		os.Exit(2)
	}
	rep := report{
		Phase:       "P7",
		Status:      "planned",
		RunID:       id,
		Profile:     strings.ToLower(strings.TrimSpace(*profile)),
		DryRun:      *dryRun,
		DatasetPlan: plan,
		RowsPlanned: rows,
		StartedAt:   start.Format(time.RFC3339),
		Guards: []string{
			"no production datasets",
			"requires performance mode",
			"requires mock external provider",
			"requires trademind_p7_ database prefix for writes",
		},
	}
	if *dryRun {
		rep.Status = "dry_run_passed"
		rep.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		write(rep)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		rep.Status = "config_failed"
		rep.Issues = append(rep.Issues, err.Error())
		write(rep)
		os.Exit(1)
	}
	if err := validateGuards(cfg); err != nil {
		rep.Status = "guard_rejected"
		rep.Issues = append(rep.Issues, err.Error())
		write(rep)
		os.Exit(1)
	}
	db, err := database.Open(cfg)
	if err != nil {
		rep.Status = "database_failed"
		rep.Issues = append(rep.Issues, err.Error())
		write(rep)
		os.Exit(1)
	}
	defer func() { _ = database.Close(db) }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.AutoMigrate(db); err != nil {
		rep.Status = "migration_failed"
		rep.Issues = append(rep.Issues, err.Error())
		write(rep)
		os.Exit(1)
	}
	summary, _ := json.Marshal(map[string]any{"profile": rep.Profile, "planned": plan, "note": "P7 foundation records the plan; bulk row materialization remains closure evidence."})
	row := &performance.TestRun{
		RunID:       id,
		Profile:     rep.Profile,
		Status:      "dataset_plan_recorded",
		DatasetRows: rows,
		StartedAt:   start,
		Summary:     datatypes.JSON(summary),
	}
	now := time.Now().UTC()
	row.FinishedAt = &now
	if err := db.WithContext(ctx).Create(row).Error; err != nil {
		rep.Status = "record_failed"
		rep.Issues = append(rep.Issues, err.Error())
		write(rep)
		os.Exit(1)
	}
	rep.Status = "dataset_plan_recorded"
	rep.RowsWritten = 1
	rep.FinishedAt = now.Format(time.RFC3339)
	write(rep)
}

func datasetPlan(profile string) (map[string]int, int64, error) {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "small":
		return map[string]int{"tenants": 10, "shops": 20, "products": 10000, "orders": 20000, "orderItems": 50000, "inventoryRows": 10000, "tasks": 20000, "webhooks": 20000, "operationLogs": 50000}, 190060, nil
	case "medium":
		return map[string]int{"tenants": 50, "shops": 100, "products": 100000, "orders": 200000, "orderItems": 500000, "inventoryRows": 100000, "tasks": 200000, "webhooks": 200000, "operationLogs": 500000}, 1800150, nil
	case "large", "stress":
		return map[string]int{"resourceBudgetRequired": 1}, 0, nil
	default:
		return nil, 0, fmt.Errorf("profile must be small, medium, large or stress")
	}
}

func validateGuards(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if config.IsProduction(cfg.AppEnv) {
		return fmt.Errorf("production dataset generation is forbidden")
	}
	if cfg.AppEnv != config.EnvPerformance {
		return fmt.Errorf("APP_ENV must be performance")
	}
	if !cfg.P7.PerformanceTestMode || !cfg.P7.AllowPerformanceDataset {
		return fmt.Errorf("PERFORMANCE_TEST_MODE and ALLOW_PERFORMANCE_DATASET must both be true")
	}
	if cfg.P7.ExternalProviderMode != "mock" {
		return fmt.Errorf("EXTERNAL_PROVIDER_MODE must be mock")
	}
	if cfg.P7.DouyinWriteEnabled || cfg.P7.AutoListingEnabled {
		return fmt.Errorf("Douyin writes and auto listing must be disabled")
	}
	if !strings.HasPrefix(strings.TrimSpace(cfg.DB.Name), "trademind_p7_") {
		return fmt.Errorf("DB_NAME must start with trademind_p7_")
	}
	return nil
}

func write(rep report) {
	b, _ := json.MarshalIndent(rep, "", "  ")
	fmt.Println(string(b))
}
