package jev

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/GinKuReNai/scythe/internal/decision"
)

// Pointer fields distinguish missing numerical answers from a valid zero.
type answer struct {
	Type          string             `json:"type"`
	Noul          *float64           `json:"noul"`
	Choice        string             `json:"choice"`
	Score         *float64           `json:"score"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    *float64           `json:"confidence"`
}
type response struct {
	ID           string            `json:"id"`
	Model        string            `json:"model"`
	ModelVersion string            `json:"model_version"`
	Answers      map[string]answer `json:"answers"`
	Usage        decision.Usage    `json:"usage"`
	LatencyMS    float64           `json:"latency_ms"`
}

func convert(data []byte) (decision.Result, error) {
	var raw response
	if err := json.Unmarshal(data, &raw); err != nil {
		return decision.Result{}, fmt.Errorf("invalid Jev JSON response")
	}
	r := decision.Result{Model: raw.Model, ModelVersion: raw.ModelVersion, DecisionID: raw.ID, Usage: raw.Usage, LatencyMS: raw.LatencyMS, RawResponse: append(json.RawMessage(nil), data...)}
	for name, target := range map[string]*float64{"runtime_reachable": &r.RuntimeReachability, "framework_required": &r.FrameworkRequired, "side_effect_risk": &r.SideEffectRisk} {
		a, ok := raw.Answers[name]
		if !ok || a.Type != "noul" || a.Noul == nil {
			return r, fmt.Errorf("missing or invalid Jev answer %s", name)
		}
		*target = *a.Noul
	}
	action := raw.Answers["action"]
	if action.Type != "choice" || len(action.Probabilities) != 3 || action.Confidence == nil {
		return r, fmt.Errorf("missing or invalid Jev action")
	}
	for _, name := range []string{"delete", "keep", "review"} {
		if _, ok := action.Probabilities[name]; !ok {
			return r, fmt.Errorf("missing Jev action probability %s", name)
		}
	}
	r.Action = action.Choice
	r.DeleteProbability = action.Probabilities["delete"]
	r.KeepProbability = action.Probabilities["keep"]
	r.ReviewProbability = action.Probabilities["review"]
	r.Confidence = *action.Confidence
	safety := raw.Answers["deletion_safety"]
	if safety.Type != "score" || safety.Score == nil || len(safety.Probabilities) != 4 {
		return r, fmt.Errorf("missing or invalid Jev deletion safety")
	}
	sum, expected := 0.0, 0.0
	for i, key := range []string{"0", "1", "2", "3"} {
		p, ok := safety.Probabilities[key]
		if !ok || p < 0 || p > 1 || math.IsNaN(p) {
			return r, fmt.Errorf("invalid Jev score distribution")
		}
		sum += p
		expected += float64(i) * p
	}
	if math.Abs(sum-1) > 0.001 || math.Abs(expected-*safety.Score) > 0.01 {
		return r, fmt.Errorf("inconsistent Jev score distribution")
	}
	r.DeletionSafety = *safety.Score
	return r, r.Validate()
}
