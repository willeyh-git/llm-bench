package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

const FileName = "llm-bencher.json"

var DefaultRuns = Runs{
	"greedy": 5,
	"4k":     5,
	"16k":    5,
	"32k":    5,
	"64k":    5,
	"96k":    2,
	"128k":   2,
}

type Runs map[string]int

func (r Runs) Count(key string) int {
	if n, ok := r[key]; ok && n > 0 {
		return n
	}
	return DefaultRuns[key]
}

type Config struct {
	BaseURL string `json:"base_url"`
	Runs    Runs   `json:"runs"`
}

func Load(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		cfg.Runs = withDefaults(nil)
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("%s: %w", path, err)
	}
	for k, v := range cfg.Runs {
		if _, ok := DefaultRuns[k]; !ok {
			return cfg, fmt.Errorf("%s: unknown runs key %q", path, k)
		}
		if v < 1 {
			return cfg, fmt.Errorf("%s: runs for %q must be at least 1", path, k)
		}
	}
	cfg.Runs = withDefaults(cfg.Runs)
	cfg.BaseURL = strings.TrimSpace(cfg.BaseURL)
	return cfg, nil
}

func Save(path string, cfg Config) error {
	cfg.Runs = withDefaults(cfg.Runs)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func withDefaults(runs Runs) Runs {
	out := Runs{}
	for k, v := range DefaultRuns {
		out[k] = v
	}
	for k, v := range runs {
		if v > 0 {
			out[k] = v
		}
	}
	return out
}
