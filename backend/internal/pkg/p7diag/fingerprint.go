package p7diag

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

const fingerprintPrefixLen = 16

var (
	reSQLString = regexp.MustCompile(`'(?:''|[^'])*'`)
	reSQLNumber = regexp.MustCompile(`\b\d+(\.\d+)?\b`)
	reSQLINList = regexp.MustCompile(`(?i)\bin\s*\((?:\s*\?\s*,)*\s*\?\s*\)`)
	reSQLSpace  = regexp.MustCompile(`\s+`)
)

// FingerprintFields are the fixed low-cardinality SQL identity fields.
type FingerprintFields struct {
	Module           string `json:"module,omitempty"`
	Operation        string `json:"operation,omitempty"`
	QueryFingerprint string `json:"queryFingerprint,omitempty"`
	QueryKind        string `json:"queryKind,omitempty"`
	TableGroup       string `json:"tableGroup,omitempty"`
	Transactional    bool   `json:"transactional,omitempty"`
}

// NormalizeSQL strips literals/parameters and collapses whitespace for safe fingerprinting.
// It never returns original parameter values.
func NormalizeSQL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	s = reSQLString.ReplaceAllString(s, "?")
	s = reSQLNumber.ReplaceAllString(s, "?")
	s = strings.ReplaceAll(s, "$1", "?")
	s = strings.ReplaceAll(s, "$2", "?")
	s = strings.ReplaceAll(s, "$3", "?")
	s = reSQLINList.ReplaceAllString(s, "IN (?)")
	s = reSQLSpace.ReplaceAllString(s, " ")
	s = strings.ToUpper(s)
	return strings.TrimSpace(s)
}

// FingerprintFromParts builds a deterministic low-cardinality fingerprint from fixed enums.
// Prefer this over hashing raw SQL that may still contain sensitive fragments.
func FingerprintFromParts(module, operation, queryKind, tableGroup string, transactional bool) FingerprintFields {
	module = strings.TrimSpace(module)
	operation = strings.TrimSpace(operation)
	queryKind = strings.TrimSpace(strings.ToLower(queryKind))
	tableGroup = strings.TrimSpace(tableGroup)
	canon := strings.Join([]string{
		module,
		operation,
		queryKind,
		tableGroup,
		boolFlag(transactional),
	}, "|")
	sum := sha256.Sum256([]byte(canon))
	return FingerprintFields{
		Module:           module,
		Operation:        operation,
		QueryFingerprint: hex.EncodeToString(sum[:])[:fingerprintPrefixLen],
		QueryKind:        queryKind,
		TableGroup:       tableGroup,
		Transactional:    transactional,
	}
}

// FingerprintFromNormalizedSQL hashes a already-normalized SQL template plus fixed operation name.
// Raw SQL with parameters must be passed through NormalizeSQL first; unknown SQL returns operation-only fingerprint.
func FingerprintFromNormalizedSQL(module, operation, queryKind, tableGroup string, transactional bool, normalizedSQL string) FingerprintFields {
	base := FingerprintFromParts(module, operation, queryKind, tableGroup, transactional)
	norm := NormalizeSQL(normalizedSQL)
	if norm == "" || looksLikeParameterizedLeak(norm) {
		return base
	}
	sum := sha256.Sum256([]byte(base.QueryFingerprint + "|" + norm))
	base.QueryFingerprint = hex.EncodeToString(sum[:])[:fingerprintPrefixLen]
	return base
}

func looksLikeParameterizedLeak(norm string) bool {
	// Reject normalized forms that still look like emails/tokens/UUIDs slipped through.
	lower := strings.ToLower(norm)
	if strings.Contains(lower, "@") {
		return true
	}
	if strings.Contains(lower, "bearer ") {
		return true
	}
	if strings.Contains(lower, "sk-") {
		return true
	}
	return false
}

func boolFlag(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func validModule(v string) bool {
	return v == "auth" || v == "webhook"
}

func validQueryKind(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "select", "insert", "update", "upsert", "delete":
		return true
	default:
		return false
	}
}

func validSQLOperation(v string) bool {
	switch v {
	case
		"auth.account_lookup",
		"auth.failed_attempt_read",
		"auth.failed_attempt_update",
		"auth.security_audit_insert",
		"auth.operation_log_chain_lookup",
		"auth.operation_log_insert",
		"webhook.event_insert",
		"webhook.idempotency_lookup",
		"webhook.idempotency_complete",
		"webhook.event_conflict_reload",
		"webhook.order_upsert",
		"webhook.task_enqueue",
		"webhook.operation_log_insert":
		return true
	default:
		return false
	}
}

func normalizeQueryError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique"):
		return "unique_violation"
	case strings.Contains(msg, "deadlock"):
		return "deadlock"
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "canceled") || strings.Contains(msg, "context deadline"):
		return "timeout"
	case strings.Contains(msg, "connection"):
		return "connection_error"
	case strings.Contains(msg, "record not found"):
		return "not_found"
	default:
		return "query_error"
	}
}

func normalizeTransactionState(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "none", "open", "committed", "rolled_back":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return "none"
	}
}
