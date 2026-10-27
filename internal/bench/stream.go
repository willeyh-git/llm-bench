package bench

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const requestTimeout = 30 * time.Minute

type Usage struct {
	PromptTokens       int `json:"prompt_tokens"`
	CompletionTokens   int `json:"completion_tokens"`
	PromptTokensDetail *struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details,omitempty"`
}

func (u *Usage) Cached() int {
	if u == nil || u.PromptTokensDetail == nil {
		return 0
	}
	return u.PromptTokensDetail.CachedTokens
}

type StreamResult struct {
	TTFT      time.Duration
	Total     time.Duration
	Usage     *Usage
	Finish    string
	Content   string
	Reasoning string
	EventAt   []time.Duration
	Chunks    int
}

func baseBody(model string, messages []Message, maxTokens int) map[string]any {
	return map[string]any{
		"model":                model,
		"messages":             messages,
		"stream":               true,
		"stream_options":       map[string]any{"include_usage": true},
		"max_tokens":           maxTokens,
		"reasoning_effort":     "none",
		"chat_template_kwargs": map[string]any{"enable_thinking": false},
	}
}

func greedyBody(model string, messages []Message, maxTokens int) map[string]any {
	body := baseBody(model, messages, maxTokens)
	body["temperature"] = 0
	body["top_p"] = 1
	body["top_k"] = 1
	body["min_p"] = 0
	body["presence_penalty"] = 0
	body["frequency_penalty"] = 0
	body["repeat_penalty"] = 1
	return body
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			Reasoning string `json:"reasoning_content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *Usage `json:"usage"`
}

func StreamChat(ctx context.Context, base string, body map[string]any) (*StreamResult, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: requestTimeout}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	out := &StreamResult{}
	var content, reasoning strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1<<20), 16<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var ev streamChunk
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return nil, fmt.Errorf("bad stream event: %w", err)
		}
		now := time.Since(start)
		if ev.Usage != nil {
			out.Usage = ev.Usage
		}
		answered := false
		for _, c := range ev.Choices {
			if c.Delta.Content != "" {
				content.WriteString(c.Delta.Content)
				answered = true
			}
			if c.Delta.Reasoning != "" {
				reasoning.WriteString(c.Delta.Reasoning)
			}
			if c.FinishReason != nil {
				out.Finish = *c.FinishReason
			}
		}
		if answered {
			out.Chunks++
			out.EventAt = append(out.EventAt, now)
			if out.TTFT == 0 {
				out.TTFT = now
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	out.Total = time.Since(start)
	out.Content = content.String()
	out.Reasoning = reasoning.String()
	return out, nil
}
