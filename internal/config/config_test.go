package config

import (
	"os"
	"path/filepath"
	"testing"
)

func noEnv(string) (string, bool) { return "", false }
func TestLoad(t *testing.T) {
	root := t.TempDir()
	c, err := Load(root, noEnv)
	if err != nil || c.Jev.Model != "jev-1.13" || !c.Cache.Enabled {
		t.Fatalf("defaults: %+v %v", c, err)
	}
	err = os.WriteFile(filepath.Join(root, ".deadcode.yaml"), []byte("version: 1\njev:\n  model: yaml-model\ncache:\n  enabled: false\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	env := func(key string) (string, bool) {
		if key == "DEADCODE_MODEL" {
			return "env-model", true
		}
		return "", false
	}
	c, err = Load(root, env)
	if err != nil || c.Jev.Model != "env-model" || c.Cache.Enabled || c.Jev.Concurrency != 8 {
		t.Fatalf("precedence/default merge: %+v %v", c, err)
	}
}
func TestRejectConfig(t *testing.T) {
	for _, text := range []string{"version: 2", "secret: abc", "jev:\n  api_key: abc", "jev:\n  concurrency: 0", "policy:\n  auto_delete_probability: 0.1", "version: 1\n---\nversion: 1"} {
		t.Run(text, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, ".deadcode.yaml"), []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(root, noEnv); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}

func TestFlagsOverrideEnvironmentAndYAML(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".deadcode.yaml"), []byte("version: 1\njev:\n  model: yaml-model\n  concurrency: 0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	model := "flag-model"
	concurrency := 2
	env := func(key string) (string, bool) {
		switch key {
		case "DEADCODE_MODEL":
			return "env-model", true
		case "DEADCODE_CONCURRENCY":
			return "invalid", true
		}
		return "", false
	}
	cfg, err := LoadWithOverrides(root, env, Overrides{Model: &model, Concurrency: &concurrency})
	if err != nil || cfg.Jev.Model != model || cfg.Jev.Concurrency != 2 {
		t.Fatalf("flag precedence failed: %+v %v", cfg, err)
	}
}
