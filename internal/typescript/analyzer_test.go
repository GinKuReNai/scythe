package typescript

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GinKuReNai/scythe/internal/analysis"
)

func TestDecodeRejectsMalformed(t *testing.T) {
	project := `{"protocolVersion":1,"type":"project","data":{"root_dir":"/repo","tsconfig_path":"/repo/tsconfig.json","framework":"node","files":0,"complete":true,"warnings":[],"dynamic_risk":false}}` + "\n"
	done := `{"protocolVersion":1,"type":"done","data":{}}` + "\n"
	for _, data := range []string{"invalid", `{"protocolVersion":2,"type":"project","data":{}}`, project, project + done + done, project + `{"protocolVersion":1,"type":"symbol","data":{}}`, project + `{"protocolVersion":1,"type":"mystery","data":{}}`, project + `{"protocolVersion":1,"type":"root","data":{"id":"unknown"}}` + "\n" + done} {
		if _, err := decode(strings.NewReader(data)); err == nil {
			t.Fatalf("accepted %q", data)
		}
	}
	if _, err := decode(bytes.NewBufferString(project + done)); err != nil {
		t.Fatal(err)
	}
}
func TestAnalyzerIntegration(t *testing.T) {
	script, err := filepath.Abs("../../analyzer/typescript/dist/index.js")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(script); err != nil {
		t.Fatal("build TypeScript analyzer before running integration tests: " + err.Error())
	}
	for _, fixture := range []string{"basic", "exports", "side-effects", "dynamic-import", "nextjs"} {
		t.Run(fixture, func(t *testing.T) {
			root, _ := filepath.Abs("../../testdata/" + fixture)
			p, err := (&Analyzer{Script: script}).Analyze(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Symbols) == 0 {
				t.Fatal("no symbols")
			}
			reachable := analysis.Reachable(p.Roots, p.Edges)
			if fixture == "basic" {
				for _, s := range p.Symbols {
					if s.Name == "deadB" && reachable[s.ID] {
						t.Fatal("dead subgraph retained")
					}
					if s.Name == "liveFunction" && !reachable[s.ID] {
						t.Fatal("live function lost")
					}
				}
			}
		})
	}
}
func TestProcessCancellation(t *testing.T) {
	script := filepath.Join(t.TempDir(), "wait.cjs")
	if err := os.WriteFile(script, []byte("setInterval(()=>{},1000)"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := (&Analyzer{Script: script}).Analyze(ctx, ".")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("subprocess cancellation too slow")
	}
}

func TestAnalyzerSecretIsolation(t *testing.T) {
	secret := "fixture-secret"
	script := filepath.Join(t.TempDir(), "secret.cjs")
	code := `if (process.env.JEV_API_KEY) { console.error('API key was inherited'); process.exit(1); }
console.log(JSON.stringify({protocolVersion:1,type:'project',data:{root_dir:'/repo',tsconfig_path:'/repo/tsconfig.json',framework:'node',files:1,complete:true,warnings:[],dynamic_risk:false}}));
console.log(JSON.stringify({protocolVersion:1,type:'symbol',data:{id:'id',name:'secret',kind:'variable',file:'index.ts',start_line:1,end_line:1,exported:false,public:false,framework_entry:false,side_effects:false,dynamic_risk:false,source:'const key = "fixture-secret";',source_truncated:false}}));
console.log(JSON.stringify({protocolVersion:1,type:'done',data:{}}));`
	if err := os.WriteFile(script, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JEV_API_KEY", secret)
	p, err := (&Analyzer{Script: script, Secret: secret}).Analyze(context.Background(), ".")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(p.Symbols[0].Source, secret) {
		t.Fatal("secret remained in source evidence")
	}
	if err = os.WriteFile(script, []byte("console.error('fixture-secret');process.exit(1)"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = (&Analyzer{Script: script, Secret: secret}).Analyze(context.Background(), ".")
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("unsafe analyzer error: %v", err)
	}
}

func TestMissingSafetyFieldsAreRejected(t *testing.T) {
	data := `{"protocolVersion":1,"type":"project","data":{"root_dir":"/repo","tsconfig_path":"/repo/tsconfig.json","framework":"node","files":1,"complete":true,"warnings":[],"dynamic_risk":false}}
{"protocolVersion":1,"type":"symbol","data":{"id":"id","name":"f","kind":"function","file":"f.ts","start_line":1,"end_line":1,"exported":false,"public":false,"framework_entry":false,"dynamic_risk":false,"source":"function f() {}","source_truncated":false}}
{"protocolVersion":1,"type":"done","data":{}}
`
	if _, err := decode(strings.NewReader(data)); err == nil {
		t.Fatal("missing side-effect evidence was accepted")
	}
}
