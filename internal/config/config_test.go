package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), FileName))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "" {
		t.Errorf("base url = %q, want empty", cfg.BaseURL)
	}
	for k, want := range DefaultRuns {
		if got := cfg.Runs.Count(k); got != want {
			t.Errorf("runs[%s] = %d, want %d", k, got, want)
		}
	}
}

func TestDefaultRunsMatchCurrentValues(t *testing.T) {
	want := map[string]int{"greedy": 5, "4k": 5, "16k": 5, "32k": 5, "64k": 5, "96k": 2, "128k": 2}
	for k, v := range want {
		if DefaultRuns[k] != v {
			t.Errorf("default runs[%s] = %d, want %d", k, DefaultRuns[k], v)
		}
	}
}

func TestLoadReadsBaseURLAndOverridesRuns(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte(`{"base_url":" http://gaming:8080/v1 ","runs":{"4k":3}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "http://gaming:8080/v1" {
		t.Errorf("base url = %q", cfg.BaseURL)
	}
	if got := cfg.Runs.Count("4k"); got != 3 {
		t.Errorf("runs[4k] = %d, want 3", got)
	}
	if got := cfg.Runs.Count("128k"); got != 2 {
		t.Errorf("runs[128k] = %d, want default 2", got)
	}
}

func TestLoadRejectsUnknownKeyAndBadCount(t *testing.T) {
	cases := map[string]string{
		"unknown key": `{"runs":{"8k":3}}`,
		"zero count":  `{"runs":{"4k":0}}`,
		"bad json":    `{"base_url":`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), FileName)
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestSaveRoundTripKeepsRuns(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := Save(path, Config{BaseURL: "http://example.test:9000/v1", Runs: Runs{"greedy": 7}}); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "http://example.test:9000/v1" {
		t.Errorf("base url = %q", cfg.BaseURL)
	}
	if got := cfg.Runs.Count("greedy"); got != 7 {
		t.Errorf("runs[greedy] = %d, want 7", got)
	}
	if got := cfg.Runs.Count("96k"); got != 2 {
		t.Errorf("runs[96k] = %d, want default 2", got)
	}
}
