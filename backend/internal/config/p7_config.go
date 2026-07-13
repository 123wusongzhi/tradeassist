package config

import (
	"fmt"
	"os"
	"strings"
)

// P7Config holds performance, capacity, pagination and limiting controls.
type P7Config struct {
	PerformanceTestMode        bool
	AllowPerformanceDataset    bool
	ExternalProviderMode       string
	DouyinWriteEnabled         bool
	AutoListingEnabled         bool
	PerformanceDatasetMaxRows  int
	PerformanceTestMaxVUs      int
	PerformanceTestMaxDuration int

	PaginationDefaultLimit int
	PaginationMaxLimit     int
	PaginationMaxOffset    int

	DBMaxOpenConnections      int
	DBMaxIdleConnections      int
	DBConnMaxLifetimeSeconds  int
	DBConnMaxIdleTimeSeconds  int
	DBQueryTimeoutMs          int
	DBTransactionTimeoutMs    int
	WorkerConcurrencyDefault  int
	WorkerQueueCapacity       int
	WorkerMaxInflight         int
	WorkerPrefetch            int
	WorkerShutdownTimeoutSecs int
	RateLimitEnabled          bool
	RateLimitMode             string
	RateLimitRedisPrefix      string
	RateLimitFailMode         string
	RateLimitLocalFallback    bool
	RateLimitPolicyVersion    string
	CacheEnabled              bool
	CacheDefaultTTLSeconds    int
	CacheMaxEntries           int
	CacheSingleflightEnabled  bool
	ExportBatchSize           int
	ExportMaxRows             int
	ExportMaxBytes            int
	ExportMaxConcurrent       int
	PprofEnabled              bool
	PprofInternalOnly         bool
}

func getenv(key string) string {
	return os.Getenv(key)
}

func loadP7Config(appEnv string) P7Config {
	return P7Config{
		PerformanceTestMode:        envBool(getenv("PERFORMANCE_TEST_MODE"), false),
		AllowPerformanceDataset:    envBool(getenv("ALLOW_PERFORMANCE_DATASET"), false),
		ExternalProviderMode:       strings.ToLower(strings.TrimSpace(firstNonEmpty(getenv("EXTERNAL_PROVIDER_MODE"), "real"))),
		DouyinWriteEnabled:         envBool(getenv("DOUYIN_WRITE_ENABLED"), false),
		AutoListingEnabled:         envBool(getenv("AUTO_LISTING_ENABLED"), false),
		PerformanceDatasetMaxRows:  atoiOrDefault(getenv("PERFORMANCE_DATASET_MAX_ROWS"), 2000000),
		PerformanceTestMaxVUs:      atoiOrDefault(getenv("PERFORMANCE_TEST_MAX_VUS"), 50),
		PerformanceTestMaxDuration: atoiOrDefault(getenv("PERFORMANCE_TEST_MAX_DURATION_SECONDS"), 1800),

		PaginationDefaultLimit: atoiOrDefault(getenv("PAGINATION_DEFAULT_LIMIT"), 50),
		PaginationMaxLimit:     atoiOrDefault(getenv("PAGINATION_MAX_LIMIT"), 200),
		PaginationMaxOffset:    atoiOrDefault(getenv("PAGINATION_MAX_OFFSET"), 10000),

		DBMaxOpenConnections:      atoiOrDefault(getenv("DB_MAX_OPEN_CONNECTIONS"), 100),
		DBMaxIdleConnections:      atoiOrDefault(getenv("DB_MAX_IDLE_CONNECTIONS"), 10),
		DBConnMaxLifetimeSeconds:  atoiOrDefault(getenv("DB_CONN_MAX_LIFETIME_SECONDS"), 3600),
		DBConnMaxIdleTimeSeconds:  atoiOrDefault(getenv("DB_CONN_MAX_IDLE_TIME_SECONDS"), 900),
		DBQueryTimeoutMs:          atoiOrDefault(getenv("DB_QUERY_TIMEOUT_MS"), 5000),
		DBTransactionTimeoutMs:    atoiOrDefault(getenv("DB_TRANSACTION_TIMEOUT_MS"), 10000),
		WorkerConcurrencyDefault:  atoiOrDefault(getenv("WORKER_CONCURRENCY_DEFAULT"), 2),
		WorkerQueueCapacity:       atoiOrDefault(getenv("WORKER_QUEUE_CAPACITY"), 1000),
		WorkerMaxInflight:         atoiOrDefault(getenv("WORKER_MAX_INFLIGHT"), 100),
		WorkerPrefetch:            atoiOrDefault(getenv("WORKER_PREFETCH"), 10),
		WorkerShutdownTimeoutSecs: atoiOrDefault(getenv("WORKER_SHUTDOWN_TIMEOUT_SECONDS"), 60),
		RateLimitEnabled:          envBool(getenv("RATE_LIMIT_ENABLED"), appEnv != EnvDevelopment),
		RateLimitMode:             strings.ToLower(strings.TrimSpace(firstNonEmpty(getenv("RATE_LIMIT_MODE"), "local"))),
		RateLimitRedisPrefix:      strings.TrimSpace(firstNonEmpty(getenv("RATE_LIMIT_REDIS_PREFIX"), "trademind:ratelimit")),
		RateLimitFailMode:         strings.ToLower(strings.TrimSpace(firstNonEmpty(getenv("RATE_LIMIT_FAIL_MODE"), "closed"))),
		RateLimitLocalFallback:    envBool(getenv("RATE_LIMIT_LOCAL_FALLBACK"), true),
		RateLimitPolicyVersion:    strings.TrimSpace(firstNonEmpty(getenv("RATE_LIMIT_POLICY_VERSION"), "p7-default-v1")),
		CacheEnabled:              envBool(getenv("CACHE_ENABLED"), true),
		CacheDefaultTTLSeconds:    atoiOrDefault(getenv("CACHE_DEFAULT_TTL_SECONDS"), 300),
		CacheMaxEntries:           atoiOrDefault(getenv("CACHE_MAX_ENTRIES"), 10000),
		CacheSingleflightEnabled:  envBool(getenv("CACHE_SINGLEFLIGHT_ENABLED"), true),
		ExportBatchSize:           atoiOrDefault(getenv("EXPORT_BATCH_SIZE"), 500),
		ExportMaxRows:             atoiOrDefault(getenv("EXPORT_MAX_ROWS"), 100000),
		ExportMaxBytes:            atoiOrDefault(getenv("EXPORT_MAX_BYTES"), 104857600),
		ExportMaxConcurrent:       atoiOrDefault(getenv("EXPORT_MAX_CONCURRENT"), 2),
		PprofEnabled:              envBool(getenv("PPROF_ENABLED"), false),
		PprofInternalOnly:         envBool(getenv("PPROF_INTERNAL_ONLY"), true),
	}
}

func (c *Config) validateP7ProductionGuards() error {
	if c == nil {
		return fmt.Errorf("%s: config is nil", ErrCodeConfigRequired)
	}
	if c.P7.PaginationDefaultLimit == 0 {
		c.P7 = loadP7Config(c.AppEnv)
	}
	p := c.P7
	if IsProduction(c.AppEnv) {
		if p.PerformanceTestMode {
			return fmt.Errorf("%s: PERFORMANCE_TEST_MODE must be false in production", ErrCodeProductionDevRouteEnabled)
		}
		if p.AllowPerformanceDataset {
			return fmt.Errorf("%s: ALLOW_PERFORMANCE_DATASET must be false in production", ErrCodeProductionDevRouteEnabled)
		}
		if p.PprofEnabled && !p.PprofInternalOnly {
			return fmt.Errorf("%s: PPROF_INTERNAL_ONLY must be true when PPROF_ENABLED=true in production", ErrCodeProductionDevRouteEnabled)
		}
		if !p.RateLimitEnabled && strings.TrimSpace(getenv("RATE_LIMIT_DISABLE_APPROVAL")) == "" {
			return fmt.Errorf("%s: RATE_LIMIT_ENABLED=false in production requires RATE_LIMIT_DISABLE_APPROVAL", ErrCodeConfigInvalid)
		}
	}
	if p.PaginationDefaultLimit < 1 || p.PaginationMaxLimit < p.PaginationDefaultLimit || p.PaginationMaxLimit > 1000 {
		return fmt.Errorf("%s: invalid pagination limits", ErrCodeConfigInvalid)
	}
	if p.PaginationMaxOffset < p.PaginationMaxLimit || p.PaginationMaxOffset > 1000000 {
		return fmt.Errorf("%s: invalid PAGINATION_MAX_OFFSET", ErrCodeConfigInvalid)
	}
	if p.ExportMaxRows < 1 || p.ExportMaxBytes < 1 || p.ExportMaxConcurrent < 1 {
		return fmt.Errorf("%s: export limits must be bounded", ErrCodeConfigInvalid)
	}
	if p.DBMaxOpenConnections < 1 || p.DBMaxIdleConnections < 0 || p.DBMaxIdleConnections > p.DBMaxOpenConnections {
		return fmt.Errorf("%s: invalid DB pool settings", ErrCodeConfigInvalid)
	}
	if p.DBConnMaxLifetimeSeconds < 60 || p.DBConnMaxIdleTimeSeconds < 30 || p.DBQueryTimeoutMs < 100 || p.DBTransactionTimeoutMs < p.DBQueryTimeoutMs {
		return fmt.Errorf("%s: invalid DB timeout settings", ErrCodeConfigInvalid)
	}
	if p.WorkerConcurrencyDefault < 1 || p.WorkerQueueCapacity < 1 || p.WorkerMaxInflight < 1 || p.WorkerPrefetch < 1 {
		return fmt.Errorf("%s: worker capacity settings must be bounded", ErrCodeConfigInvalid)
	}
	if p.CacheEnabled && (p.CacheDefaultTTLSeconds < 1 || p.CacheMaxEntries < 1) {
		return fmt.Errorf("%s: cache limits must be bounded", ErrCodeConfigInvalid)
	}
	switch p.RateLimitMode {
	case "local", "redis", "hybrid":
	default:
		return fmt.Errorf("%s: RATE_LIMIT_MODE must be local, redis or hybrid", ErrCodeConfigInvalid)
	}
	switch p.RateLimitFailMode {
	case "closed", "local_fallback":
	default:
		return fmt.Errorf("%s: RATE_LIMIT_FAIL_MODE must be closed or local_fallback", ErrCodeConfigInvalid)
	}
	if p.PerformanceTestMode {
		if !p.AllowPerformanceDataset {
			return fmt.Errorf("%s: PERFORMANCE_TEST_MODE requires ALLOW_PERFORMANCE_DATASET=true", ErrCodeConfigInvalid)
		}
		if p.ExternalProviderMode != "mock" {
			return fmt.Errorf("%s: PERFORMANCE_TEST_MODE requires EXTERNAL_PROVIDER_MODE=mock", ErrCodeConfigInvalid)
		}
		if p.DouyinWriteEnabled || p.AutoListingEnabled {
			return fmt.Errorf("%s: performance mode forbids Douyin writes and auto listing", ErrCodeConfigInvalid)
		}
	}
	return nil
}
