package scan

import (
	"github.com/GinKuReNai/scythe/internal/analysis"
	"github.com/GinKuReNai/scythe/internal/decision"
)

type Finding struct {
	Candidate
	Decision     *decision.Result `json:"decision,omitempty"`
	Outcome      string           `json:"outcome"`
	PolicyReason string           `json:"policy_reason"`
	Cached       bool             `json:"cached"`
}
type Statistics struct {
	Files        int `json:"files"`
	Symbols      int `json:"symbols"`
	Reachable    int `json:"reachable"`
	Candidates   int `json:"candidates"`
	SafeToDelete int `json:"safe_to_delete"`
	Review       int `json:"review"`
	Kept         int `json:"kept"`
	CacheHits    int `json:"cache_hits"`
	ModelCalls   int `json:"model_calls"`
}
type Result struct {
	SchemaVersion int              `json:"schema_version"`
	Project       analysis.Project `json:"project"`
	Statistics    Statistics       `json:"statistics"`
	Findings      []Finding        `json:"findings"`
}
