package safeenv

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type DBConfig struct {
	URL              string
	Driver           string
	HostCategory     string
	DatabaseName     string
	DatabaseNameHash string
}

type RedisConfig struct {
	URL string
}

func TestDatabaseURLFromEnv() (DBConfig, bool, error) {
	raw := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if raw == "" {
		return DBConfig{}, false, nil
	}
	if normalizeEnvironment(os.Getenv("APP_ENV")) != "test" {
		return DBConfig{}, true, fmt.Errorf("APP_ENV=test is required for PostgreSQL integration tests")
	}
	u, err := parseTestDatabaseURL(raw)
	if err != nil {
		return DBConfig{}, true, err
	}
	dbName := strings.Trim(strings.ToLower(u.Path), "/")
	sum := sha256.Sum256([]byte(dbName))
	return DBConfig{
		URL:              raw,
		Driver:           "postgresql",
		HostCategory:     testHostCategory(u.Hostname()),
		DatabaseName:     dbName,
		DatabaseNameHash: hex.EncodeToString(sum[:]),
	}, true, nil
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
	_, err := parseTestDatabaseURL(raw)
	return err
}

func ValidateActualTestDatabaseName(expected, actual string) error {
	expected = strings.TrimSpace(strings.ToLower(expected))
	actual = strings.TrimSpace(strings.ToLower(actual))
	if expected == "" || actual == "" || expected != actual {
		return fmt.Errorf("Unsafe PostgreSQL Test Database Rejected: actual database does not match TEST_DATABASE_URL")
	}
	return validateTestDatabaseName(actual)
}

func parseTestDatabaseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("TEST_DATABASE_URL must be a valid postgres URL")
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return nil, fmt.Errorf("TEST_DATABASE_URL must use postgres/postgresql scheme")
	}
	dbName := strings.Trim(strings.ToLower(u.Path), "/")
	if err := validateTestDatabaseName(dbName); err != nil {
		return nil, err
	}
	if !isLocalTestHost(u.Hostname()) && !enabled(os.Getenv("P9_POSTGRES_ALLOW_REMOTE_TEST_HOST")) {
		return nil, fmt.Errorf("remote PostgreSQL test host requires P9_POSTGRES_ALLOW_REMOTE_TEST_HOST=1")
	}
	return u, nil
}

func validateTestDatabaseName(dbName string) error {
	dbName = strings.TrimSpace(strings.ToLower(dbName))
	if dbName == "" || !hasTestMarker(dbName) {
		return fmt.Errorf("TEST_DATABASE_URL database name must contain a bounded test or e2e marker")
	}
	for _, token := range strings.FieldsFunc(dbName, func(r rune) bool { return r == '_' || r == '-' || r == '.' }) {
		switch token {
		case "prod", "production", "staging", "stage", "main", "master":
			return fmt.Errorf("Unsafe PostgreSQL Test Database Rejected")
		}
	}
	switch dbName {
	case "postgres", "trademind", "template0", "template1":
		return fmt.Errorf("Unsafe PostgreSQL Test Database Rejected")
	}
	return nil
}

func testHostCategory(host string) string {
	if isLocalTestHost(host) {
		return "local"
	}
	return "remote_test"
}

func isLocalTestHost(host string) bool {
	switch strings.TrimSpace(strings.ToLower(host)) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func normalizeEnvironment(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func enabled(value string) bool {
	switch normalizeEnvironment(value) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
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
	for _, token := range strings.FieldsFunc(strings.ToLower(value), func(r rune) bool { return r == '_' || r == '-' || r == '.' }) {
		if token == "test" || token == "tests" || token == "e2e" || token == "ci" {
			return true
		}
	}
	return false
}
