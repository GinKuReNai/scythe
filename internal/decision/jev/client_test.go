package jev

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/GinKuReNai/scythe/internal/analysis"
)

const goodResponse = `{"id":"dec_fixture","model":"jev-1.13","model_version":"jev-1.13-test","answers":{"runtime_reachable":{"type":"noul","noul":0.01},"framework_required":{"type":"noul","noul":0.01},"side_effect_risk":{"type":"noul","noul":0.01},"deletion_safety":{"type":"score","score":2.95,"probabilities":{"0":0,"1":0,"2":0.05,"3":0.95},"confidence":0.95},"action":{"type":"choice","choice":"delete","probabilities":{"delete":0.99,"keep":0,"review":0.01},"confidence":0.99}},"usage":{"input_tokens":123,"output_tokens":10,"charged_tokens":123,"wallet":"tokens"},"latency_ms":123.4}`

func TestClientSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer fixture-secret" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("incorrect request headers")
		}
		var body struct {
			Model     string            `json:"model"`
			State     analysis.Evidence `json:"state"`
			Questions map[string]struct {
				Type     string          `json:"type"`
				Criteria json.RawMessage `json:"criteria"`
			} `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Model != "jev-1.13" || body.State.SchemaVersion != 1 || len(body.Questions) != 5 {
			t.Errorf("incorrect request: %+v", body)
		}
		for name, q := range body.Questions {
			switch q.Type {
			case "noul":
				if len(q.Criteria) != 0 {
					t.Errorf("noul criteria: %s", name)
				}
			case "choice":
				if q.Criteria[0] != '{' {
					t.Error("choice must be object")
				}
			case "score":
				if q.Criteria[0] != '[' {
					t.Error("score must be array")
				}
			}
		}
		fmt.Fprint(w, goodResponse)
	}))
	defer server.Close()
	result, err := NewClient(server.Client(), server.URL, "jev-1.13", "fixture-secret").Decide(context.Background(), analysis.Evidence{SchemaVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.DeletionSafety != 2.95 || result.DeleteProbability != .99 || result.DecisionID != "dec_fixture" || result.Usage.InputTokens != 123 || result.LatencyMS != 123.4 {
		t.Fatalf("lost response metadata: %+v", result)
	}
}
func TestRejectResponse(t *testing.T) {
	for _, body := range []string{"invalid", `{}`, strings.Replace(goodResponse, `"noul":0.01`, `"noul":2`, 1), strings.Replace(goodResponse, `"noul":0.01`, `"missing":0.01`, 1), strings.Replace(goodResponse, `"score":2.95`, `"score":1.5`, 1), strings.Replace(goodResponse, `"delete":0.99`, `"delete":0.5`, 1), strings.Replace(goodResponse, `"type":"choice"`, `"type":"noul"`, 1)} {
		if _, err := convert([]byte(body)); err == nil {
			t.Fatalf("accepted malformed response: %s", body)
		}
	}
}
func TestErrorsNeverEchoSecrets(t *testing.T) {
	for _, status := range []int{401, 429, 500, 302} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status); fmt.Fprint(w, "fixture-secret") }))
			defer server.Close()
			_, err := NewClient(server.Client(), server.URL, "model", "fixture-secret").Decide(context.Background(), analysis.Evidence{})
			if err == nil || strings.Contains(err.Error(), "fixture-secret") {
				t.Fatalf("unsafe error: %v", err)
			}
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, strings.Replace(goodResponse, "dec_fixture", "fixture-secret", 1))
	}))
	defer server.Close()
	if _, err := NewClient(server.Client(), server.URL, "model", "fixture-secret").Decide(context.Background(), analysis.Evidence{}); err == nil || strings.Contains(err.Error(), "fixture-secret") {
		t.Fatalf("credential response accepted: %v", err)
	}
}
func TestCancellationAndTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(200 * time.Millisecond):
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewClient(server.Client(), server.URL, "model", "key").Decide(ctx, analysis.Evidence{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	_, err = NewClient(&http.Client{Timeout: 10 * time.Millisecond}, server.URL, "model", "key").Decide(context.Background(), analysis.Evidence{})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("timeout: %v", err)
	}
	_, err = NewClient(nil, server.URL, "model", "").Decide(context.Background(), analysis.Evidence{})
	if err == nil || !strings.Contains(err.Error(), "JEV_API_KEY") {
		t.Fatal("missing credential not diagnosed")
	}
}
