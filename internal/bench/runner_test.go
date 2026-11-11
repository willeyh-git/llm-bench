package bench

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/willyhetland/llm-bencher/internal/config"
)

func fakeServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/models") {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":[{"id":"Gemma-4-12B","context_length":262144,"status":{"value":"loaded"}}]}`)
			return
		}
		var body map[string]any
		decodeJSON(t, r, &body)
		w.Header().Set("Content-Type", "text/event-stream")
		maxTokens, _ := body["max_tokens"].(float64)
		if maxTokens == decodeMaxTokens {
			writeDecodeStream(w, body)
			return
		}
		writePrefillStream(w, body)
	}))
}

func promptTokens(body map[string]any) int {
	msgs, _ := body["messages"].([]any)
	if len(msgs) == 0 {
		return 10
	}
	sys, ok := msgs[0].(map[string]any)
	if !ok {
		return 10
	}
	c, _ := sys["content"].(string)
	return int(float64(len([]rune(c))) / 4.25)
}

func writePrefillStream(w http.ResponseWriter, body map[string]any) {
	fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":null}]}\n\n")
	fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":%d,\"completion_tokens\":1,\"prompt_tokens_details\":{\"cached_tokens\":0}}}\n\n", promptTokens(body))
	fmt.Fprint(w, "data: [DONE]\n\n")
}

func writeDecodeStream(w http.ResponseWriter, body map[string]any) {
	for i := 0; i < decodeMaxTokens; i++ {
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"x\"},\"finish_reason\":null}]}\n\n")
	}
	fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"length\"}],\"usage\":{\"prompt_tokens\":%d,\"completion_tokens\":512}}\n\n", promptTokens(body))
	fmt.Fprint(w, "data: [DONE]\n\n")
}

func TestRunnerEndToEndEventOrderAndSchema(t *testing.T) {
	srv := fakeServer(t)
	defer srv.Close()

	out := filepath.Join(t.TempDir(), "run.json")
	events := make(chan Event, 4)
	runner := &Runner{Base: srv.URL + "/v1", Model: "Gemma-4-12B", Out: out, Events: events}

	var starts []string
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for ev := range events {
			if ev.Kind == EventPhaseStart {
				starts = append(starts, ev.Phase)
			}
		}
	}()

	res, err := runner.Run(context.Background())
	close(events)
	<-drained
	if err != nil {
		t.Fatal(err)
	}
	want := []string{PhaseWarmup, PhaseDecodeGreedy, PhaseDecode4K, PhaseDecode16K, PhaseDecode32K, PhaseDecode64K, PhaseDecode96K, PhaseDecode128K}
	if strings.Join(starts, ",") != strings.Join(want, ",") {
		t.Fatalf("phase order = %v, want %v", starts, want)
	}

	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("result file not written: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"schema", "model", "started_at", "prefill", "decode", "decode_at_context", "corpus"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("result JSON missing %q", key)
		}
	}
	if decoded["schema"] != float64(resultSchema) {
		t.Errorf("schema = %v", decoded["schema"])
	}
	if decoded["corpus"] != CorpusSHA256 {
		t.Errorf("corpus = %v", decoded["corpus"])
	}
	prefill, _ := decoded["prefill"].(map[string]any)
	for _, k := range []string{"4000", "16000", "32000", "64000", "96000", "126000"} {
		if _, ok := prefill[k]; !ok {
			t.Errorf("prefill missing %s", k)
		}
	}
	if len(res.Decode.Runs) != decodeRuns {
		t.Errorf("decode runs = %d, want %d", len(res.Decode.Runs), decodeRuns)
	}
	for _, d := range decodeSizes {
		dc, ok := res.DecodeAtContext[fmt.Sprint(d.target)]
		if !ok {
			t.Errorf("decode at %d missing", d.target)
			continue
		}
		if want := config.DefaultRuns.Count(d.key); len(dc.Runs) != want {
			t.Errorf("decode at %d runs = %d, want %d", d.target, len(dc.Runs), want)
		}
		pr, ok := res.Prefill[fmt.Sprint(d.target)]
		if !ok {
			t.Errorf("prefill at %d missing: it must come from the decode runs", d.target)
			continue
		}
		if want := config.DefaultRuns.Count(d.key); len(pr.TTFTSamples) != want {
			t.Errorf("prefill at %d ttft samples = %d, want %d", d.target, len(pr.TTFTSamples), want)
		}
		if pr.TTFTSeconds <= 0 || pr.PromptTokens <= 0 {
			t.Errorf("prefill at %d has no timing: %+v", d.target, pr)
		}
	}
	if res.Path != out {
		t.Errorf("path = %q, want %q", res.Path, out)
	}
}

func TestFitsContextAllowsForPromptOvershoot(t *testing.T) {
	cases := []struct {
		target, out, ctxLen int
		want                bool
	}{
		{126000, 256, 128000, false},
		{126000, 256, 102400, false},
		{126000, 256, 143360, true},
		{64000, 512, 102400, true},
		{16000, 256, 32768, true},
		{64000, 512, 32768, false},
		{126000, 256, 0, true},
	}
	for _, c := range cases {
		if got := fitsContext(c.target, c.out, c.ctxLen); got != c.want {
			t.Errorf("fitsContext(%d, %d, %d) = %v, want %v", c.target, c.out, c.ctxLen, got, c.want)
		}
	}
}

func TestRunnerSkipsPrefillRejectedByServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/models") {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":[{"id":"Qwen3.8-27B","context_length":143360,"status":{"value":"loaded"}}]}`)
			return
		}
		var body map[string]any
		decodeJSON(t, r, &body)
		msgs, _ := body["messages"].([]any)
		if len(msgs) > 0 {
			if sys, ok := msgs[0].(map[string]any); ok {
				if c, _ := sys["content"].(string); len([]rune(c)) > 500000 {
					http.Error(w, `{"error":"request (131069 tokens) exceeds the available context size (128000 tokens), try increasing it"}`, http.StatusBadRequest)
					return
				}
			}
		}
		w.Header().Set("Content-Type", "text/event-stream")
		maxTokens, _ := body["max_tokens"].(float64)
		if maxTokens == decodeMaxTokens {
			writeDecodeStream(w, body)
			return
		}
		writePrefillStream(w, body)
	}))
	defer srv.Close()

	events := make(chan Event, 64)
	var errorPhases []string
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for ev := range events {
			if ev.Kind == EventError {
				errorPhases = append(errorPhases, ev.Phase)
			}
		}
	}()

	runner := &Runner{Base: srv.URL + "/v1", Model: "Qwen3.8-27B", Out: filepath.Join(t.TempDir(), "run.json"), Events: events}
	res, err := runner.Run(context.Background())
	close(events)
	<-drained
	if err != nil {
		t.Fatalf("run must complete when the server rejects one size: %v", err)
	}
	if len(errorPhases) != 0 {
		t.Errorf("server rejection must not emit error events, got %v", errorPhases)
	}
	reason, ok := res.Skipped["126000"]
	if !ok || !strings.Contains(reason, "server rejected") {
		t.Errorf("126000 must be skipped with a server rejection reason, got %q", reason)
	}
	for _, k := range []string{"4000", "16000", "64000"} {
		if _, ok := res.Prefill[k]; !ok {
			t.Errorf("prefill %s missing", k)
		}
	}
	if _, ok := res.Prefill["126000"]; ok {
		t.Error("prefill 126000 must not be recorded when the server rejects it")
	}
}

func TestRunnerUsesConfiguredRunCounts(t *testing.T) {
	srv := fakeServer(t)
	defer srv.Close()

	out := filepath.Join(t.TempDir(), "run.json")
	runs := config.Runs{"greedy": 3, "4k": 1}
	runner := &Runner{Base: srv.URL + "/v1", Model: "Gemma-4-12B", Out: out, ContextLength: 262144, Runs: runs}
	res, err := runner.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Decode.Runs) != 3 {
		t.Errorf("greedy decode runs = %d, want 3", len(res.Decode.Runs))
	}
	if dc := res.DecodeAtContext["4000"]; len(dc.Runs) != 1 {
		t.Errorf("decode at 4000 runs = %d, want 1", len(dc.Runs))
	}
	if dc := res.DecodeAtContext["16000"]; len(dc.Runs) != config.DefaultRuns.Count("16k") {
		t.Errorf("decode at 16000 runs = %d, want default %d", len(dc.Runs), config.DefaultRuns.Count("16k"))
	}
}

func TestRunnerRunsOnlySelectedContexts(t *testing.T) {
	srv := fakeServer(t)
	defer srv.Close()
	runner := &Runner{Base: srv.URL + "/v1", Model: "Gemma-4-12B", Out: filepath.Join(t.TempDir(), "selected.json"), ContextLength: 262144, Contexts: []string{"4k", "16k"}, Runs: config.Runs{"greedy": 1, "4k": 1, "16k": 1}}
	res, err := runner.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Prefill) != 2 || len(res.DecodeAtContext) != 2 {
		t.Fatalf("selected tiers recorded: prefill=%v decode=%v", res.Prefill, res.DecodeAtContext)
	}
	for _, key := range []string{"32000", "64000", "96000", "126000"} {
		if _, ok := res.Prefill[key]; ok {
			t.Errorf("unselected prefill tier %s was run", key)
		}
	}
}

func TestRunnerSkipsSizesAboveContextLength(t *testing.T) {
	srv := fakeServer(t)
	defer srv.Close()

	out := filepath.Join(t.TempDir(), "skip.json")
	events := make(chan Event, 16)
	runner := &Runner{Base: srv.URL + "/v1", Model: "Devstral Small 2 24B", Out: out, ContextLength: 32768, Events: events}

	var skipped []string
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for ev := range events {
			if ev.Kind == EventSkipped {
				skipped = append(skipped, ev.Phase)
			}
		}
	}()

	res, err := runner.Run(context.Background())
	close(events)
	<-drained
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := res.Prefill["16000"]; !ok {
		t.Error("16K prefill fits 32768 and must run")
	}
	for _, k := range []string{"32000", "64000", "96000", "126000"} {
		if _, ok := res.Prefill[k]; ok {
			t.Errorf("prefill %s must be skipped above context_length", k)
		}
		if _, ok := res.Skipped[k]; !ok {
			t.Errorf("prefill %s missing skip reason", k)
		}
	}
	for _, k := range []string{"4000", "16000"} {
		if _, ok := res.DecodeAtContext[k]; !ok {
			t.Errorf("decode at %s fits 32768 and must run", k)
		}
	}
	for _, k := range []string{"32000", "64000", "96000", "126000"} {
		if _, ok := res.DecodeAtContext[k]; ok {
			t.Errorf("decode at %s must be skipped above context_length", k)
		}
		if _, ok := res.DecodeAtContextSkipped[k]; !ok {
			t.Errorf("decode at %s missing skip reason", k)
		}
	}
	want := []string{PhaseDecode32K, PhaseDecode64K, PhaseDecode96K, PhaseDecode128K}
	if strings.Join(skipped, ",") != strings.Join(want, ",") {
		t.Fatalf("skipped phases = %v, want %v", skipped, want)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("result file not written: %v", err)
	}
}

func TestRunnerCancelWritesNoFile(t *testing.T) {
	srv := fakeServer(t)
	defer srv.Close()

	out := filepath.Join(t.TempDir(), "cancelled.json")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runner := &Runner{Base: srv.URL + "/v1", Model: "Gemma-4-12B", Out: out}

	if _, err := runner.Run(ctx); err == nil {
		t.Fatal("expected error on cancelled context")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("result file must not exist after cancel, stat err = %v", err)
	}
}

func TestRunnerServerDownWritesNoFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	srv.Close()

	out := filepath.Join(t.TempDir(), "down.json")
	runner := &Runner{Base: srv.URL + "/v1", Model: "m", Out: out}
	if _, err := runner.Run(context.Background()); err == nil {
		t.Fatal("expected error when server is unreachable")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("no partial result may be written")
	}
}

func TestSlug(t *testing.T) {
	if got := Slug("Devstral Small 2 24B"); got != "devstral-small-2-24b" {
		t.Fatalf("slug = %q", got)
	}
	if got := Slug("PolyStrata: Qwen3.6 35B-A3B"); got != "polystrata-qwen3-6-35b-a3b" {
		t.Fatalf("slug = %q", got)
	}
}
