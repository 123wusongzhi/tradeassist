package safeenv

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type DBConfig struct {
	URL string
}

type RedisConfig struct {
	URL string
}

func TestDatabaseURLFromEnv() (DBConfig, bool, error) {
	raw := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if raw == "" {
		return DBConfig{}, false, nil
	}
	if err := ValidateTestDatabaseURL(raw); err != nil {
		return DBConfig{}, true, err
	}
	return DBConfig{URL: raw}, true, nil
}

func TestRedisURLFromEnv() (RedisConfig, bool, error) {
	raw := strings.TrimSpace(os.Getenv("TEST_REDIS_URL"))
	if raw == "" {
		return RedisConfig{}, false, nil
	}
	if err := ValidateTestRedisURL(raw); err != nil {
		return RedisConfig{}, true, err
	}
	return RedisConfig{URL: raw}, true, nil
}

func ValidateTestDatabaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("TEST_DATABASE_URL must be a valid postgres URL")
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return fmt.Errorf("TEST_DATABASE_URL must use postgres/postgresql scheme")
	}
	dbName := strings.Trim(strings.ToLower(u.Path), "/")
	if !hasTestMarker(dbName) {
		return fmt.Errorf("TEST_DATABASE_URL database name must contain test, _test, or e2e")
	}
	return nil
}

func ValidateTestRedisURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("TEST_REDIS_URL must be a valid redis URL")
	}
	if u.Scheme != "redis" && u.Scheme != "rediss" {
		return fmt.Errorf("TEST_REDIS_URL must use redis/rediss scheme")
	}
	path := strings.Trim(u.Path, "/")
	if path == "" {
		return fmt.Errorf("TEST_REDIS_URL must include an isolated test DB number")
	}
	db, err := strconv.Atoi(path)
	if err != nil || db <= 0 {
		return fmt.Errorf("TEST_REDIS_URL DB must be a positive isolated test DB number")
	}
	return nil
}

func hasTestMarker(value string) bool {
	return strings.Contains(value, "test") || strings.Contains(value, "_test") || strings.Contains(value, "e2e")
}
