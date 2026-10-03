package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GinKuReNai/scythe/internal/analysis"
	"github.com/GinKuReNai/scythe/internal/config"
	"github.com/GinKuReNai/scythe/internal/policy"
	"github.com/GinKuReNai/scythe/internal/scan"
)

type analyzerFake struct{}

func (analyzerFake) Analyze(context.Context, string) (*analysis.Project, error) {
	return &analysis.Project{Complete: true, Framework: "node", Files: 1, Symbols: []analysis.Symbol{{ID: "index.ts:0:function", Name: "dead", Kind: "function", File: "index.ts", StartLine: 1, EndLine: 1, Source: "function dead() {}"}}}, nil
}
func testDependencies() Dependencies {
	return Dependencies{Version: "test", LoadConfig: func(_ string, o config.Overrides) (config.Config, error) {
		c := config.Defaults()
		if o.Format != nil {
			c.Format = *o.Format
		}
		if o.Model != nil {
			c.Jev.Model = *o.Model
		}
		if o.Concurrency != nil {
			c.Jev.Concurrency = *o.Concurrency
		}
		return c, c.Validate()
	}, Scanner: func(_ context.Context, c config.Config, _ bool) (*scan.Service, func() error, error) {
		return scan.NewService(analyzerFake{}, nil, nil, policy.Defaults(), c.Jev.Model, "q1", c.Jev.Concurrency), func() error { return nil }, nil
	}, Clean: func(context.Context, config.Config) error { return nil }}
}
func execute(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var out, logs bytes.Buffer
	cmd := NewRootCommand(testDependencies())
	cmd.SetOut(&out)
	cmd.SetErr(&logs)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(context.Background())
	return out.String(), logs.String(), err
}
func TestCommands(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"version"}, {"cache", "clean"}, {"explain", "dead", "--offline"}} {
		out, _, err := execute(t, args...)
		if err != nil || out == "" {
			t.Fatalf("%v: %v %q", args, err, out)
		}
	}
}
func TestTextGolden(t *testing.T) {
	out, logs, err := execute(t, "scan", "--offline")
	if err != nil {
		t.Fatal(err)
	}
	if logs != "" {
		t.Fatalf("unexpected stderr: %s", logs)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "scan.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if out != string(want) {
		t.Fatalf("text output changed:\n%s", out)
	}
}
func TestJSONAndExplain(t *testing.T) {
	out, logs, err := execute(t, "scan", "--format=json", "--offline")
	if err != nil {
		t.Fatal(err)
	}
	var result scan.Result
	if err = json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if logs != "" || result.SchemaVersion != 1 || len(result.Findings) != 1 || result.Findings[0].Outcome != policy.Review {
		t.Fatalf("invalid output: %s", out)
	}
	out, _, err = execute(t, "explain", "index.ts:0:function", "--format=json")
	if err != nil || !strings.Contains(out, "function dead()") {
		t.Fatalf("explain: %s %v", out, err)
	}
	if _, _, err = execute(t, "explain", "unknown"); err == nil {
		t.Fatal("unknown symbol accepted")
	}
	if _, _, err = execute(t, "scan", "--format=invalid"); err == nil {
		t.Fatal("invalid format accepted")
	}
}
