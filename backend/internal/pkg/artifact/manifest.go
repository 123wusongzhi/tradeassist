package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/pkg/backupruntime"
)

// ReleaseManifest is the P6 release metadata contract. It intentionally excludes secrets.
type ReleaseManifest struct {
	ReleaseID                  string            `json:"releaseId"`
	Version                    string            `json:"version"`
	GitCommit                  string            `json:"gitCommit"`
	GitTreeState               string            `json:"gitTreeState"`
	BuiltAt                    string            `json:"builtAt"`
	GoVersion                  string            `json:"goVersion"`
	NodeVersion                string            `json:"nodeVersion"`
	PNPMVersion                string            `json:"pnpmVersion"`
	BackendSHA256              string            `json:"backendSha256"`
	AdminSHA256                string            `json:"adminSha256"`
	CollectorSHA256            string            `json:"collectorSha256"`
	MigrationVersion           string            `json:"migrationVersion"`
	MinimumCompatibleSchema    string            `json:"minimumCompatibleSchema"`
	MaximumCompatibleSchema    string            `json:"maximumCompatibleSchema"`
	ConfigurationSchemaVersion string            `json:"configurationSchemaVersion"`
	RollbackCompatible         bool              `json:"rollbackCompatible"`
	RequiredFeatures           []string          `json:"requiredFeatures"`
	Artifacts                  []Artifact        `json:"artifacts"`
	Dependencies               map[string]string `json:"dependencies,omitempty"`
	ManifestSHA256             string            `json:"manifestSha256,omitempty"`
}

// Artifact describes one packaged output.
type Artifact struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// BuildReleaseManifest calculates artifact hashes and a manifest hash.
func BuildReleaseManifest(releaseID, version, gitCommit, treeState string, paths map[string]string) (*ReleaseManifest, error) {
	m := &ReleaseManifest{
		ReleaseID:                  releaseID,
		Version:                    version,
		GitCommit:                  gitCommit,
		GitTreeState:               treeState,
		BuiltAt:                    time.Now().UTC().Format(time.RFC3339),
		GoVersion:                  runtime.Version(),
		RollbackCompatible:         true,
		ConfigurationSchemaVersion: "p6-v1",
		RequiredFeatures:           []string{},
		Artifacts:                  []Artifact{},
		Dependencies:               map[string]string{},
	}
	for name, path := range paths {
		sum, size, err := backupruntime.SHA256File(path)
		if err != nil {
			return nil, err
		}
		a := Artifact{Name: name, Path: path, Size: size, SHA256: sum}
		m.Artifacts = append(m.Artifacts, a)
		switch name {
		case "backend":
			m.BackendSHA256 = sum
		case "admin":
			m.AdminSHA256 = sum
		case "collector":
			m.CollectorSHA256 = sum
		}
	}
	if err := m.RefreshHash(); err != nil {
		return nil, err
	}
	return m, nil
}

// RefreshHash recalculates the manifest SHA-256 over its JSON without the hash field.
func (m *ReleaseManifest) RefreshHash() error {
	if m == nil {
		return fmt.Errorf("release manifest is nil")
	}
	old := m.ManifestSHA256
	m.ManifestSHA256 = ""
	raw, err := json.Marshal(m)
	if err != nil {
		m.ManifestSHA256 = old
		return err
	}
	sum := sha256.Sum256(raw)
	m.ManifestSHA256 = hex.EncodeToString(sum[:])
	return nil
}

// Write writes the manifest with 0600 permissions.
func (m *ReleaseManifest) Write(path string) error {
	if err := m.RefreshHash(); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}
