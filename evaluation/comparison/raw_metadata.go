package main

// raw_metadata.go records reproducibility metadata alongside the raw
// dataset. Collection is best-effort: if git or Go version information is
// unavailable, the corresponding field is left empty rather than failing the
// export. The benchmark itself never depends on this metadata.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// DatasetMetadata documents the exact configuration and environment used to
// produce the raw dataset, for reproducibility and provenance.
type DatasetMetadata struct {
	PeerCounts       []int    `json:"peer_counts"`
	Seeds            []int64  `json:"seeds"`
	Implementations  []string `json:"implementations"`
	Messages         int      `json:"messages"`
	MessageSizeBytes int      `json:"message_size_bytes"`
	PublishRate      float64  `json:"publish_rate"`
	FLRounds         int      `json:"fl_rounds"`

	GeneratedAt time.Time `json:"generated_at"`
	GitCommit   string    `json:"git_commit,omitempty"`
	GoVersion   string    `json:"go_version"`
	OS          string    `json:"os"`
	Arch        string    `json:"arch"`
}

func buildDatasetMetadata(cfg ExperimentConfig) DatasetMetadata {
	implNames := make([]string, 0, len(rawDatasetImplementations()))
	for _, name := range rawDatasetImplementations() {
		implNames = append(implNames, rawDatasetDisplayName(name))
	}
	return DatasetMetadata{
		PeerCounts:       cfg.PeerCounts,
		Seeds:            rawDatasetSeeds(cfg),
		Implementations:  implNames,
		Messages:         cfg.Messages,
		MessageSizeBytes: cfg.MessageSize,
		PublishRate:      cfg.PublishRate,
		FLRounds:         cfg.FLRounds,
		GeneratedAt:      time.Now().UTC(),
		GitCommit:        gitCommit(),
		GoVersion:        runtime.Version(),
		OS:               runtime.GOOS,
		Arch:             runtime.GOARCH,
	}
}

// gitCommit best-effort resolves the current commit hash. It returns an
// empty string (never an error) if git is unavailable or the working tree is
// not a git repository -- metadata collection must never fail the export.
func gitCommit() string {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func writeMetadataJSON(path string, metadata DatasetMetadata) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
