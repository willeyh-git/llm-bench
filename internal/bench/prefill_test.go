package bench

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func decodeJSON(t *testing.T, r *http.Request, v any) {
	t.Helper()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		t.Errorf("decode request body: %v", err)
	}
}

func sseServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, body)
	}))
}

const streamWithUsage = `data: {"choices":[{"delta":{"role":"assistant"},"finish_reason":null}]}

data: {"choices":[{"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"choices":[{"delta":{"content":" world"},"finish_reason":"stop"}]}

data: {"choices":[],"usage":{"prompt_tokens":4133,"completion_tokens":2,"prompt_tokens_details":{"cached_tokens":0}}}

data: [DONE]

`

func TestStreamChatParsesTTFTAndUsage(t *testing.T) {
	srv := sseServer(t, streamWithUsage)
	defer srv.Close()

	res, err := StreamChat(context.Background(), srv.URL+"/v1", greedyBody("m", nil, 8))
	if err != nil {
		t.Fatal(err)
	}
	if res.Content != "Hello world" {
		t.Errorf("content = %q", res.Content)
	}
	if res.Finish != "stop" {
		t.Errorf("finish = %q", res.Finish)
	}
	if res.Usage == nil || res.Usage.PromptTokens != 4133 {
		t.Fatalf("usage = %+v", res.Usage)
	}
	if res.TTFT <= 0 || res.TTFT > res.Total {
		t.Errorf("ttft %v not within total %v", res.TTFT, res.Total)
	}
	if res.Chunks != 2 {
		t.Errorf("answer chunks = %d, want 2", res.Chunks)
	}
}

func TestStreamChatTTFTIgnoresRoleOnlyEvent(t *testing.T) {
	srv := sseServer(t, streamWithUsage)
	defer srv.Close()

	res, err := StreamChat(context.Background(), srv.URL+"/v1", greedyBody("m", nil, 8))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.EventAt) != 2 {
		t.Fatalf("content events = %d, want 2", len(res.EventAt))
	}
	if res.TTFT != res.EventAt[0] {
		t.Errorf("ttft %v is not the first content event %v", res.TTFT, res.EventAt[0])
	}
}

func TestStreamChatSeparatesReasoning(t *testing.T) {
	body := `data: {"choices":[{"delta":{"reasoning_content":"thinking..."},"finish_reason":null}]}

data: {"choices":[{"delta":{"content":"answer"},"finish_reason":"stop"}]}

data: [DONE]

`
	srv := sseServer(t, body)
	defer srv.Close()

	res, err := StreamChat(context.Background(), srv.URL+"/v1", greedyBody("m", nil, 8))
	if err != nil {
		t.Fatal(err)
	}
	if res.Reasoning != "thinking..." || res.Content != "answer" {
		t.Fatalf("reasoning=%q content=%q", res.Reasoning, res.Content)
	}
}

func TestStreamChatHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not found", http.StatusNotFound)
	}))
	defer srv.Close()

	if _, err := StreamChat(context.Background(), srv.URL+"/v1", greedyBody("m", nil, 8)); err == nil {
		t.Fatal("expected error on HTTP 404")
	}
}

func TestStreamChatRequestSendsThinkingOffAndUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got map[string]any
		decodeJSON(t, r, &got)
		if got["stream"] != true {
			t.Error("stream not enabled")
		}
		opts, _ := got["stream_options"].(map[string]any)
		if opts["include_usage"] != true {
			t.Error("include_usage not set")
		}
		kwargs, _ := got["chat_template_kwargs"].(map[string]any)
		if kwargs["enable_thinking"] != false {
			t.Error("enable_thinking not disabled")
		}
		if got["reasoning_effort"] != "none" {
			t.Error("reasoning_effort not none")
		}
		if got["top_k"] != float64(1) || got["temperature"] != float64(0) {
			t.Errorf("greedy settings missing: %v", got)
		}
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	if _, err := StreamChat(context.Background(), srv.URL+"/v1", greedyBody("m", nil, 8)); err != nil {
		t.Fatal(err)
	}
}

func TestMeasurePrefillValidTiming(t *testing.T) {
	srv := sseServer(t, streamWithUsage)
	defer srv.Close()

	res, err := MeasurePrefill(context.Background(), srv.URL+"/v1", "Gemma-4-12B", 4000)
	if err != nil {
		t.Fatal(err)
	}
	if res.PromptTokens != 4133 {
		t.Errorf("prompt tokens = %d", res.PromptTokens)
	}
	if res.TPS <= 0 {
		t.Errorf("tps = %v", res.TPS)
	}
	if len(res.Invalid) != 0 {
		t.Errorf("unexpected invalid reasons: %v", res.Invalid)
	}
}

func TestValidatePrefillRules(t *testing.T) {
	ttft := 500 * time.Millisecond
	tests := []struct {
		name     string
		model    string
		target   int
		usage    *Usage
		ttft     time.Duration
		wantInv  string
		wantFlag string
		wantOK   bool
	}{
		{name: "missing usage", model: "Gemma", target: 4000, usage: nil, ttft: ttft, wantInv: "usage.prompt_tokens missing"},
		{name: "cached prefix", model: "Gemma", target: 4000, usage: &Usage{PromptTokens: 4133, PromptTokensDetail: &struct {
			CachedTokens int `json:"cached_tokens"`
		}{CachedTokens: 2000}}, ttft: ttft, wantInv: "cached_tokens"},
		{name: "qwen token count off", model: "Qwen3.6 35B", target: 4000, usage: &Usage{PromptTokens: 4300}, ttft: ttft, wantInv: "outside ±1%"},
		{name: "qwen token count ok", model: "Qwen3.6 35B", target: 4000, usage: &Usage{PromptTokens: 4140}, ttft: ttft, wantOK: true},
		{name: "non-qwen unchecked", model: "Gemma-4-12B", target: 4000, usage: &Usage{PromptTokens: 9999}, ttft: ttft, wantFlag: "unchecked", wantOK: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stream := &StreamResult{TTFT: tc.ttft, Usage: tc.usage}
			res := validatePrefill(tc.model, tc.target, stream)
			if tc.wantOK && len(res.Invalid) != 0 {
				t.Fatalf("unexpected invalid: %v", res.Invalid)
			}
			if tc.wantInv != "" && !anyContains(res.Invalid, tc.wantInv) {
				t.Fatalf("invalid %v does not mention %q", res.Invalid, tc.wantInv)
			}
			if tc.wantFlag != "" && !anyContains(res.Flags, tc.wantFlag) {
				t.Fatalf("flags %v do not mention %q", res.Flags, tc.wantFlag)
			}
		})
	}
}

func anyContains(items []string, sub string) bool {
	for _, s := range items {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
