package config

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/GinKuReNai/scythe/internal/policy"
	"go.yaml.in/yaml/v3"
)

type Config struct {
	Version  int `yaml:"version"`
	Analysis struct {
		Exclude []string `yaml:"exclude"`
	} `yaml:"analysis"`
	Jev struct {
		Model          string `yaml:"model"`
		Concurrency    int    `yaml:"concurrency"`
		TimeoutSeconds int    `yaml:"timeout_seconds"`
	} `yaml:"jev"`
	Policy policy.Config `yaml:"policy"`
	Cache  struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"cache"`
	Format   string `yaml:"-"`
	Analyzer string `yaml:"-"`
	CacheDir string `yaml:"-"`
}

func Defaults() Config {
	c := Config{Version: 1, Policy: policy.Defaults(), Format: "text"}
	c.Analysis.Exclude = []string{"**/*.generated.ts", "**/node_modules/**"}
	c.Jev.Model = "jev-1.13"
	c.Jev.Concurrency = 8
	c.Jev.TimeoutSeconds = 30
	c.Cache.Enabled = true
	return c
}

// Load reads strict YAML followed by explicit environment overrides. Secrets
// intentionally have no configuration field.
type Overrides struct {
	Model, Format, Analyzer *string
	Concurrency             *int
	CacheEnabled            *bool
}

func Load(root string, lookup func(string) (string, bool)) (Config, error) {
	return LoadWithOverrides(root, lookup, Overrides{})
}
func LoadWithOverrides(root string, lookup func(string) (string, bool), overrides Overrides) (Config, error) {
	c := Defaults()
	data, err := os.ReadFile(filepath.Join(root, ".deadcode.yaml"))
	if err != nil && !os.IsNotExist(err) {
		return c, fmt.Errorf("read .deadcode.yaml: %w", err)
	}
	if err == nil {
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(true)
		if err = decoder.Decode(&c); err != nil {
			return c, fmt.Errorf("parse .deadcode.yaml: %w", err)
		}
		var extra any
		if err = decoder.Decode(&extra); err != io.EOF {
			return c, fmt.Errorf(".deadcode.yaml must contain one document")
		}
	}
	for key, target := range map[string]*string{"DEADCODE_MODEL": &c.Jev.Model, "DEADCODE_FORMAT": &c.Format, "DEADCODE_ANALYZER": &c.Analyzer, "DEADCODE_CACHE_DIR": &c.CacheDir} {
		override := map[string]*string{"DEADCODE_MODEL": overrides.Model, "DEADCODE_FORMAT": overrides.Format, "DEADCODE_ANALYZER": overrides.Analyzer}[key]
		if override != nil {
			*target = *override
		} else if value, ok := lookup(key); ok {
			*target = value
		}
	}
	if overrides.Concurrency != nil {
		c.Jev.Concurrency = *overrides.Concurrency
	} else if value, ok := lookup("DEADCODE_CONCURRENCY"); ok {
		n, err := strconv.Atoi(value)
		if err != nil {
			return c, fmt.Errorf("DEADCODE_CONCURRENCY must be an integer")
		}
		c.Jev.Concurrency = n
	}
	if overrides.CacheEnabled != nil {
		c.Cache.Enabled = *overrides.CacheEnabled
	}
	return c, c.Validate()
}
func (c Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported configuration version %d", c.Version)
	}
	if strings.TrimSpace(c.Jev.Model) == "" {
		return fmt.Errorf("jev.model must not be empty")
	}
	if c.Jev.Concurrency < 1 || c.Jev.Concurrency > 64 {
		return fmt.Errorf("jev.concurrency must be between 1 and 64")
	}
	if c.Jev.TimeoutSeconds < 1 || c.Jev.TimeoutSeconds > 600 {
		return fmt.Errorf("jev.timeout_seconds must be between 1 and 600")
	}
	if c.Format != "text" && c.Format != "json" {
		return fmt.Errorf("format must be text or json")
	}
	for _, v := range []float64{c.Policy.AutoDeleteProbability, c.Policy.MaxRuntimeReachability, c.Policy.MaxFrameworkRequiredProbability, c.Policy.MaxSideEffectRisk} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
			return fmt.Errorf("policy probabilities must be between 0 and 1")
		}
	}
	if c.Policy.AutoDeleteProbability < 0.9 {
		return fmt.Errorf("auto_delete_probability must be at least 0.9")
	}
	if math.IsNaN(c.Policy.MinDeletionSafety) || math.IsInf(c.Policy.MinDeletionSafety, 0) || c.Policy.MinDeletionSafety < 0 || c.Policy.MinDeletionSafety > 3 {
		return fmt.Errorf("min_deletion_safety must be between 0 and 3")
	}
	return nil
}
