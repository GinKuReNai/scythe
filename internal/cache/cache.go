package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/GinKuReNai/scythe/internal/analysis"
	"github.com/GinKuReNai/scythe/internal/decision"
)

const SchemaVersion = 1

func Key(e analysis.Evidence, model, questions string) (string, error) {
	payload := struct {
		CacheVersion    int               `json:"cache_version"`
		EvidenceVersion int               `json:"evidence_version"`
		DecisionVersion string            `json:"decision_version"`
		QuestionVersion string            `json:"question_version"`
		Model           string            `json:"model"`
		Evidence        analysis.Evidence `json:"evidence"`
	}{SchemaVersion, analysis.EvidenceVersion, decision.SemanticsVersion, questions, model, e}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode cache key: %w", err)
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}
