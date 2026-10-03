package policy

import (
	"testing"

	"github.com/GinKuReNai/scythe/internal/analysis"
	"github.com/GinKuReNai/scythe/internal/decision"
)

func safeDecision() decision.Result {
	return decision.Result{Action: "delete", DeleteProbability: 0.99, ReviewProbability: 0.01, RuntimeReachability: 0.01, FrameworkRequired: 0.01, SideEffectRisk: 0.01, DeletionSafety: 2.95, Confidence: 0.99, Model: "test", ModelVersion: "v1", DecisionID: "id"}
}
func TestPolicy(t *testing.T) {
	cases := []struct {
		name   string
		change func(*analysis.Evidence, *decision.Result)
		want   string
	}{
		{"safe", func(*analysis.Evidence, *decision.Result) {}, SafeToDelete},
		{"delete below threshold", func(_ *analysis.Evidence, r *decision.Result) { r.DeleteProbability = .979; r.ReviewProbability = .021 }, Review},
		{"delete at threshold", func(_ *analysis.Evidence, r *decision.Result) { r.DeleteProbability = .98; r.ReviewProbability = .02 }, SafeToDelete},
		{"runtime boundary", func(_ *analysis.Evidence, r *decision.Result) { r.RuntimeReachability = .05 }, Review},
		{"framework boundary", func(_ *analysis.Evidence, r *decision.Result) { r.FrameworkRequired = .05 }, Review},
		{"side effect boundary", func(_ *analysis.Evidence, r *decision.Result) { r.SideEffectRisk = .05 }, Review},
		{"score below", func(_ *analysis.Evidence, r *decision.Result) { r.DeletionSafety = 2.899 }, Review},
		{"score boundary", func(_ *analysis.Evidence, r *decision.Result) { r.DeletionSafety = 2.9 }, SafeToDelete},
		{"public", func(e *analysis.Evidence, _ *decision.Result) { e.Symbol.Public = true }, Keep},
		{"framework entry", func(e *analysis.Evidence, _ *decision.Result) { e.Symbol.FrameworkEntry = true }, Keep},
		{"reachable", func(e *analysis.Evidence, _ *decision.Result) { e.Analysis.Reachable = true }, Keep},
		{"exported", func(e *analysis.Evidence, _ *decision.Result) { e.Symbol.Exported = true }, Review},
		{"static side effects", func(e *analysis.Evidence, _ *decision.Result) { e.Symbol.SideEffects = true }, Review},
		{"dynamic", func(e *analysis.Evidence, _ *decision.Result) { e.DynamicRisk = true }, Review},
		{"incomplete", func(e *analysis.Evidence, _ *decision.Result) { e.Complete = false }, Review},
		{"truncated", func(e *analysis.Evidence, _ *decision.Result) { e.Symbol.SourceTruncated = true }, Review},
		{"keep", func(_ *analysis.Evidence, r *decision.Result) {
			r.Action = "keep"
			r.DeleteProbability = .01
			r.KeepProbability = .99
			r.ReviewProbability = 0
		}, Keep},
		{"review action", func(_ *analysis.Evidence, r *decision.Result) { r.Action = "review" }, Review},
		{"invalid", func(_ *analysis.Evidence, r *decision.Result) { r.DeleteProbability = 2 }, Review},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			e := analysis.Evidence{Complete: true}
			r := safeDecision()
			tt.change(&e, &r)
			got, _ := Defaults().Apply(e, &r)
			if got != tt.want {
				t.Fatalf("got %s want %s", got, tt.want)
			}
		})
	}
	got, _ := Defaults().Apply(analysis.Evidence{Complete: true}, nil)
	if got != Review {
		t.Fatal("nil model result must be reviewed")
	}
}
