package decision

import (
	"encoding/json"
	"fmt"
	"math"
)

const SemanticsVersion = "1"

type Result struct {
	Action              string          `json:"action"`
	DeleteProbability   float64         `json:"delete_probability"`
	KeepProbability     float64         `json:"keep_probability"`
	ReviewProbability   float64         `json:"review_probability"`
	RuntimeReachability float64         `json:"runtime_reachability"`
	FrameworkRequired   float64         `json:"framework_required"`
	SideEffectRisk      float64         `json:"side_effect_risk"`
	DeletionSafety      float64         `json:"deletion_safety"`
	Confidence          float64         `json:"confidence"`
	Model               string          `json:"model"`
	ModelVersion        string          `json:"model_version"`
	DecisionID          string          `json:"decision_id"`
	Usage               Usage           `json:"usage"`
	LatencyMS           float64         `json:"latency_ms"`
	RawResponse         json.RawMessage `json:"-"`
}
type Usage struct {
	InputTokens    int     `json:"input_tokens"`
	OutputTokens   int     `json:"output_tokens"`
	ChargedTokens  int     `json:"charged_tokens"`
	ChargedCredits float64 `json:"charged_credits"`
	Wallet         string  `json:"wallet"`
}

func (r Result) Validate() error {
	if r.Action != "delete" && r.Action != "keep" && r.Action != "review" {
		return fmt.Errorf("invalid decision action")
	}
	for _, p := range []float64{r.DeleteProbability, r.KeepProbability, r.ReviewProbability, r.RuntimeReachability, r.FrameworkRequired, r.SideEffectRisk, r.Confidence} {
		if math.IsNaN(p) || math.IsInf(p, 0) || p < 0 || p > 1 {
			return fmt.Errorf("invalid decision probability")
		}
	}
	if math.Abs(r.DeleteProbability+r.KeepProbability+r.ReviewProbability-1) > 0.001 {
		return fmt.Errorf("action probabilities must sum to one")
	}
	if math.IsNaN(r.DeletionSafety) || math.IsInf(r.DeletionSafety, 0) || r.DeletionSafety < 0 || r.DeletionSafety > 3 {
		return fmt.Errorf("invalid deletion safety score")
	}
	if r.Model == "" || r.ModelVersion == "" || r.DecisionID == "" {
		return fmt.Errorf("missing decision metadata")
	}
	return nil
}
