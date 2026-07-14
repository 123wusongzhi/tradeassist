package main

import (
	"context"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/pkg/providerlimit"
)

type providerAdaptiveReport struct {
	Phase                  string   `json:"phase"`
	Status                 string   `json:"status"`
	GeneratedAt            string   `json:"generatedAt"`
	SlowdownObserved       bool     `json:"slowdownObserved"`
	RetryAfterRespected    bool     `json:"retryAfterRespected"`
	GradualRecovery        bool     `json:"gradualRecovery"`
	RateNeverZero          bool     `json:"rateNeverZero"`
	ContextCancelRespected bool     `json:"contextCancelRespected"`
	BeforeConcurrency      int      `json:"beforeConcurrency"`
	AfterSlowdown          int      `json:"afterSlowdown"`
	AfterRecovery          int      `json:"afterRecovery"`
	DurationMs             int64    `json:"durationMs"`
	Guards                 []string `json:"guards"`
	Issues                 []string `json:"issues"`
}

func runProviderAdaptive(ctx context.Context) (providerAdaptiveReport, error) {
	started := time.Now().UTC()
	if err := validateGuardsFromEnv(); err != nil {
		return providerAdaptiveReport{}, err
	}
	_ = ctx

	rep := providerAdaptiveReport{
		Phase:       phase,
		Status:      "passed",
		GeneratedAt: started.Format(time.RFC3339),
		Guards:      guardList(),
	}

	reg := providerlimit.NewRegistry(providerlimit.Config{
		DefaultConcurrency: 8,
		MaxWait:            time.Second,
		RecoveryInterval:   5 * time.Millisecond,
	})
	reg.Observe(providerlimit.ProviderAI, providerlimit.OperationText, 200, 0, nil)
	before := reg.Snapshot(providerlimit.ProviderAI, providerlimit.OperationText)
	rep.BeforeConcurrency = before.Concurrency

	reg.Observe(providerlimit.ProviderAI, providerlimit.OperationText, 429, 15*time.Millisecond, nil)
	slow := reg.Snapshot(providerlimit.ProviderAI, providerlimit.OperationText)
	rep.AfterSlowdown = slow.Concurrency
	rep.SlowdownObserved = slow.Concurrency < before.Concurrency
	rep.RetryAfterRespected = slow.RetryAfterRespected
	rep.RateNeverZero = slow.RateNeverZero

	waitCtx, cancel := context.WithTimeout(context.Background(), 2*time.Millisecond)
	defer cancel()
	_, waitErr := reg.Acquire(waitCtx, providerlimit.ProviderAI, providerlimit.OperationText)
	rep.ContextCancelRespected = waitErr != nil

	time.Sleep(25 * time.Millisecond)
	reg.Observe(providerlimit.ProviderAI, providerlimit.OperationText, 200, 0, nil)
	reg.Observe(providerlimit.ProviderAI, providerlimit.OperationText, 200, 0, nil)
	recovered := reg.Snapshot(providerlimit.ProviderAI, providerlimit.OperationText)
	rep.AfterRecovery = recovered.Concurrency
	rep.GradualRecovery = recovered.Concurrency > slow.Concurrency && recovered.RecoveryIsGradual

	if !rep.SlowdownObserved {
		rep.Status = "failed"
		rep.Issues = append(rep.Issues, "adaptive slowdown was not observed after 429")
	}
	if !rep.GradualRecovery {
		rep.Status = "failed"
		rep.Issues = append(rep.Issues, "gradual recovery was not observed")
	}
	if !rep.RateNeverZero {
		rep.Status = "failed"
		rep.Issues = append(rep.Issues, "effective concurrency dropped to zero")
	}

	rep.DurationMs = time.Since(started).Milliseconds()
	return rep, nil
}
