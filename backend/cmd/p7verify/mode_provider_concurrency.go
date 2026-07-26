package main

import (
	"context"
	"sync"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/pkg/providerlimit"
)

type providerConcurrencyReport struct {
	Phase                string   `json:"phase"`
	Status               string   `json:"status"`
	GeneratedAt          string   `json:"generatedAt"`
	AcquireReleaseOK     bool     `json:"acquireReleaseOk"`
	BoundedWaitOK        bool     `json:"boundedWaitOk"`
	MaxInflightRespected bool     `json:"maxInflightRespected"`
	DoubleReleaseSafe    bool     `json:"doubleReleaseSafe"`
	RegistryEntries      int      `json:"registryEntries"`
	DurationMs           int64    `json:"durationMs"`
	Guards               []string `json:"guards"`
	Issues               []string `json:"issues"`
}

func runProviderConcurrency(ctx context.Context) (providerConcurrencyReport, error) {
	started := time.Now().UTC()
	if err := validateGuardsFromEnv(); err != nil {
		return providerConcurrencyReport{}, err
	}
	_ = ctx

	rep := providerConcurrencyReport{
		Phase:       phase,
		Status:      "passed",
		GeneratedAt: started.Format(time.RFC3339),
		Guards:      guardList(),
	}

	reg := providerlimit.NewRegistry(providerlimit.Config{
		DefaultConcurrency: 2,
		MaxWait:            20 * time.Millisecond,
		MaxEntries:         16,
	})

	l1, err := reg.Acquire(context.Background(), providerlimit.ProviderDouyinShop, providerlimit.OperationRead)
	if err != nil {
		rep.Status = "failed"
		rep.Issues = append(rep.Issues, err.Error())
		return rep, nil
	}
	l2, err := reg.Acquire(context.Background(), providerlimit.ProviderDouyinShop, providerlimit.OperationRead)
	if err != nil {
		l1.Release()
		rep.Status = "failed"
		rep.Issues = append(rep.Issues, err.Error())
		return rep, nil
	}
	rep.AcquireReleaseOK = true

	waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if _, err := reg.Acquire(waitCtx, providerlimit.ProviderDouyinShop, providerlimit.OperationRead); err == nil {
		rep.BoundedWaitOK = false
		rep.Status = "failed"
		rep.Issues = append(rep.Issues, "expected bounded wait failure when concurrency exhausted")
	} else {
		rep.BoundedWaitOK = true
	}

	l1.Release()
	l2.Release()
	l1.Release()
	rep.DoubleReleaseSafe = true
	if _, err := reg.Acquire(context.Background(), providerlimit.ProviderDouyinShop, providerlimit.OperationRead); err != nil {
		rep.AcquireReleaseOK = false
		rep.Status = "failed"
		rep.Issues = append(rep.Issues, "acquire after release failed: "+err.Error())
	}

	reg2 := providerlimit.NewRegistry(providerlimit.Config{DefaultConcurrency: 2, MaxWait: time.Second})
	start := make(chan struct{})
	var wg sync.WaitGroup
	inflight := 0
	maxInflight := 0
	var mu sync.Mutex
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			lease, err := reg2.Acquire(context.Background(), providerlimit.ProviderAI, providerlimit.OperationText)
			if err != nil {
				return
			}
			mu.Lock()
			inflight++
			if inflight > maxInflight {
				maxInflight = inflight
			}
			mu.Unlock()
			time.Sleep(time.Millisecond)
			mu.Lock()
			inflight--
			mu.Unlock()
			lease.Release()
		}()
	}
	close(start)
	wg.Wait()
	rep.MaxInflightRespected = maxInflight <= 2
	if !rep.MaxInflightRespected {
		rep.Status = "failed"
		rep.Issues = append(rep.Issues, "max inflight exceeded configured concurrency")
	}

	snap := reg.Snapshot(providerlimit.ProviderDouyinShop, providerlimit.OperationRead)
	if snap.Concurrency > 0 {
		rep.RegistryEntries = 1
	}

	rep.DurationMs = time.Since(started).Milliseconds()
	return rep, nil
}
