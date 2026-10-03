package scan

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GinKuReNai/scythe/internal/analysis"
	"github.com/GinKuReNai/scythe/internal/cache/sqlite"
	"github.com/GinKuReNai/scythe/internal/decision"
	"github.com/GinKuReNai/scythe/internal/policy"
)

type fakeAnalyzer struct{ project *analysis.Project }

func (f fakeAnalyzer) Analyze(ctx context.Context, _ string) (*analysis.Project, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return f.project, nil
}

type fakeEngine struct{ calls, active, maximum atomic.Int32 }

func (f *fakeEngine) Decide(ctx context.Context, e analysis.Evidence) (decision.Result, error) {
	f.calls.Add(1)
	active := f.active.Add(1)
	defer f.active.Add(-1)
	for {
		old := f.maximum.Load()
		if active <= old || f.maximum.CompareAndSwap(old, active) {
			break
		}
	}
	select {
	case <-ctx.Done():
		return decision.Result{}, ctx.Err()
	case <-time.After(10 * time.Millisecond):
	}
	return decision.Result{Action: "delete", DeleteProbability: .99, ReviewProbability: .01, DeletionSafety: 2.95, Model: "test", ModelVersion: "v1", DecisionID: e.Symbol.ID}, nil
}
func testProject() *analysis.Project {
	return &analysis.Project{Complete: true, Files: 1, Symbols: []analysis.Symbol{{ID: "live", Name: "live", Kind: "function"}, {ID: "deadA", Name: "deadA", Kind: "function"}, {ID: "deadB", Name: "deadB", Kind: "function"}, {ID: "export", Name: "publicUnknown", Exported: true}, {ID: "side", Name: "side", SideEffects: true}}, Roots: []string{"live"}, Edges: []analysis.Reference{{From: "deadA", To: "deadB"}}}
}
func TestCandidatesAndFilters(t *testing.T) {
	candidates, live := FindCandidates(testProject())
	if live != 1 || len(candidates) != 4 {
		t.Fatalf("got %d live, %d candidates", live, len(candidates))
	}
	for _, c := range candidates {
		if c.Evidence.Symbol.ID == "deadB" && c.Evidence.Analysis.DirectReferenceCount != 1 {
			t.Fatal("dead subgraph evidence lost")
		}
	}
}
func TestScanPersistenceAndBoundedCalls(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "cache.db")
	store, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	engine := &fakeEngine{}
	service := NewService(fakeAnalyzer{testProject()}, engine, store, policy.Defaults(), "test", "q1", 1)
	first, err := service.Scan(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	if first.Statistics.ModelCalls != 2 || first.Statistics.SafeToDelete != 2 || first.Statistics.Review != 2 || engine.calls.Load() != 2 || engine.maximum.Load() > 1 {
		t.Fatalf("first scan: %+v", first.Statistics)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service = NewService(fakeAnalyzer{testProject()}, engine, store, policy.Defaults(), "test", "q1", 2)
	second, err := service.Scan(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	if second.Statistics.CacheHits != 2 || engine.calls.Load() != 2 {
		t.Fatalf("cache not used: %+v", second.Statistics)
	}
	changed := testProject()
	changed.Symbols[1].Source = "changed"
	service = NewService(fakeAnalyzer{changed}, engine, store, policy.Defaults(), "test", "q1", 2)
	third, err := service.Scan(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	if third.Statistics.ModelCalls != 1 || third.Statistics.CacheHits != 1 {
		t.Fatalf("evidence change not invalidated: %+v", third.Statistics)
	}
}
func TestOfflineAndCancellation(t *testing.T) {
	service := NewService(fakeAnalyzer{testProject()}, nil, nil, policy.Defaults(), "test", "q1", 2)
	result, err := service.Scan(context.Background(), ".")
	if err != nil {
		t.Fatal(err)
	}
	if result.Statistics.Review != 4 {
		t.Fatal("offline must review")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = service.Scan(ctx, "."); err == nil {
		t.Fatal("cancellation ignored")
	}
}

func TestExplainOnlyEvaluatesMatchingSymbol(t *testing.T) {
	engine := &fakeEngine{}
	service := NewService(fakeAnalyzer{testProject()}, engine, nil, policy.Defaults(), "test", "q1", 2)
	result, err := service.Explain(context.Background(), ".", "deadA")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 1 || engine.calls.Load() != 1 {
		t.Fatal("explain evaluated unrelated candidates")
	}
	result, err = service.Explain(context.Background(), ".", "live")
	if err != nil {
		t.Fatal(err)
	}
	if result.Findings[0].Outcome != policy.Keep || engine.calls.Load() != 1 {
		t.Fatal("reachable symbol was sent to model")
	}
}

func TestUnknownExternalCallersProtectExportDependencies(t *testing.T) {
	p := &analysis.Project{Complete: true, Symbols: []analysis.Symbol{{ID: "export", Name: "export", Exported: true}, {ID: "helper", Name: "helper"}}, Edges: []analysis.Reference{{From: "export", To: "helper"}}}
	engine := &fakeEngine{}
	service := NewService(fakeAnalyzer{p}, engine, nil, policy.Defaults(), "test", "q1", 2)
	result, err := service.Scan(context.Background(), ".")
	if err != nil {
		t.Fatal(err)
	}
	if result.Statistics.Review != 2 || engine.calls.Load() != 0 {
		t.Fatal("export dependencies were trusted for deletion")
	}
	for _, f := range result.Findings {
		if !f.Evidence.Analysis.PotentialExternalReachability {
			t.Fatal("external reachability evidence missing")
		}
	}
}
