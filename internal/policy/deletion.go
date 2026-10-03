package policy

import (
	"github.com/GinKuReNai/scythe/internal/analysis"
	"github.com/GinKuReNai/scythe/internal/decision"
)

const (
	SafeToDelete = "SAFE_TO_DELETE"
	Keep         = "KEEP"
	Review       = "REVIEW"
)

type Config struct {
	AutoDeleteProbability           float64 `yaml:"auto_delete_probability"`
	MaxRuntimeReachability          float64 `yaml:"max_runtime_reachability"`
	MaxFrameworkRequiredProbability float64 `yaml:"max_framework_required_probability"`
	MaxSideEffectRisk               float64 `yaml:"max_side_effect_risk"`
	MinDeletionSafety               float64 `yaml:"min_deletion_safety"`
}

func Defaults() Config { return Config{0.98, 0.05, 0.05, 0.05, 2.9} }

// StaticOutcome applies hard safeguards before consulting a model.
func StaticOutcome(e analysis.Evidence) (string, string) {
	if e.Analysis.Reachable || e.Symbol.Public || e.Symbol.FrameworkEntry {
		return Keep, "reachable or required public/framework surface"
	}
	if e.Symbol.SideEffects {
		return Review, "declaration may have observable side effects"
	}
	if !e.Complete {
		return Review, "analysis is incomplete"
	}
	if e.DynamicRisk {
		return Review, "dynamic usage may evade static references"
	}
	if e.Analysis.PotentialExternalReachability {
		return Review, "symbol may be reached through exports with unknown external callers"
	}
	if e.Symbol.Exported {
		return Review, "export may have external consumers"
	}
	if e.Symbol.SourceTruncated || e.Analysis.ReferencesTruncated {
		return Review, "source evidence is truncated"
	}
	return "", ""
}
func (c Config) Apply(e analysis.Evidence, r *decision.Result) (string, string) {
	if outcome, reason := StaticOutcome(e); outcome != "" {
		return outcome, reason
	}
	if r == nil {
		return Review, "no model decision available"
	}
	if r.Validate() != nil {
		return Review, "invalid model decision"
	}
	if r.Action == "keep" && r.KeepProbability >= 0.9 {
		return Keep, "model evidence favors keeping the symbol"
	}
	if r.Action == "delete" && r.DeleteProbability >= c.AutoDeleteProbability && r.RuntimeReachability < c.MaxRuntimeReachability && r.FrameworkRequired < c.MaxFrameworkRequiredProbability && r.SideEffectRisk < c.MaxSideEffectRisk && r.DeletionSafety >= c.MinDeletionSafety {
		return SafeToDelete, "all conservative policy thresholds passed"
	}
	return Review, "model decision did not meet conservative policy thresholds"
}
