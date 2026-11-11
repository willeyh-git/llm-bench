package bench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/willyhetland/llm-bencher/internal/config"
	"github.com/willyhetland/llm-bencher/internal/server"
)

const resultSchema = 2

const promptOvershootPercent = 5

type Event struct {
	Kind  string
	Phase string
	Data  any
}

const (
	EventPhaseStart = "phase_start"
	EventProgress   = "progress"
	EventPhaseEnd   = "phase_end"
	EventDone       = "done"
	EventError      = "error"
	EventSkipped    = "skipped"
)

const (
	PhaseWarmup       = "warmup"
	PhaseDecodeGreedy = "decode_greedy"
	PhaseDecode4K     = "decode_4k"
	PhaseDecode16K    = "decode_16k"
	PhaseDecode32K    = "decode_32k"
	PhaseDecode64K    = "decode_64k"
	PhaseDecode96K    = "decode_96k"
	PhaseDecode128K   = "decode_128k"
)

var decodeSizes = []struct {
	phase  string
	key    string
	target int
}{
	{PhaseDecode4K, "4k", 4000},
	{PhaseDecode16K, "16k", 16000},
	{PhaseDecode32K, "32k", 32000},
	{PhaseDecode64K, "64k", 64000},
	{PhaseDecode96K, "96k", 96000},
	{PhaseDecode128K, "128k", 128000},
}

type Result struct {
	Schema                 int                      `json:"schema"`
	Model                  string                   `json:"model"`
	StartedAt              time.Time                `json:"started_at"`
	Prefill                map[string]PrefillResult `json:"prefill"`
	Skipped                map[string]string        `json:"skipped,omitempty"`
	Decode                 DecodeResult             `json:"decode"`
	DecodeAtContext        map[string]DecodeResult  `json:"decode_at_context"`
	DecodeAtContextSkipped map[string]string        `json:"decode_at_context_skipped,omitempty"`
	Corpus                 string                   `json:"corpus"`
	Path                   string                   `json:"-"`
}

type Runner struct {
	Base, Model   string
	Out           string
	ContextLength int
	Runs          config.Runs
	Events        chan<- Event
}

func fitsContext(target, outTokens, contextLength int) bool {
	if contextLength <= 0 {
		return true
	}
	estimatedPrompt := target + target*promptOvershootPercent/100
	return estimatedPrompt+outTokens <= contextLength
}

func isContextOverflow(err error) bool {
	return strings.Contains(err.Error(), "exceeds the available context size")
}

func lookupContextLength(ctx context.Context, base, model string) int {
	models, err := server.ListModels(ctx, base)
	if err != nil {
		return 0
	}
	for _, m := range models {
		if m.ID == model {
			return m.ContextLength
		}
	}
	return 0
}

type skipError struct {
	reason string
}

func (e skipError) Error() string {
	return e.reason
}

func skipReason(target, outTokens, contextLength int) string {
	return fmt.Sprintf("skipped: %d-token prompt and %d output tokens exceed context_length %d", target, outTokens, contextLength)
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func Slug(model string) string {
	return strings.Trim(slugRe.ReplaceAllString(strings.ToLower(model), "-"), "-")
}

func (r *Runner) emit(ctx context.Context, e Event) {
	if r.Events == nil {
		return
	}
	select {
	case r.Events <- e:
	case <-ctx.Done():
	}
}

func (r *Runner) phase(ctx context.Context, name string, fn func() (any, error)) (any, error) {
	r.emit(ctx, Event{Kind: EventPhaseStart, Phase: name})
	data, err := fn()
	if err != nil {
		if isContextOverflow(err) {
			reason := "skipped: server rejected the prompt: " + err.Error()
			r.emit(ctx, Event{Kind: EventSkipped, Phase: name, Data: reason})
			return nil, skipError{reason: reason}
		}
		r.emit(ctx, Event{Kind: EventError, Phase: name, Data: err.Error()})
		return nil, err
	}
	r.emit(ctx, Event{Kind: EventPhaseEnd, Phase: name, Data: data})
	return data, nil
}

func (r *Runner) Run(ctx context.Context) (*Result, error) {
	if err := VerifyCorpus(); err != nil {
		return nil, err
	}
	res := &Result{
		Schema:                 resultSchema,
		Model:                  r.Model,
		StartedAt:              time.Now().UTC(),
		Prefill:                map[string]PrefillResult{},
		Skipped:                map[string]string{},
		DecodeAtContext:        map[string]DecodeResult{},
		DecodeAtContextSkipped: map[string]string{},
		Corpus:                 CorpusSHA256,
	}

	if r.ContextLength == 0 {
		r.ContextLength = lookupContextLength(ctx, r.Base, r.Model)
	}

	if _, err := r.phase(ctx, PhaseWarmup, func() (any, error) {
		return nil, Warmup(ctx, r.Base, r.Model)
	}); err != nil {
		return nil, err
	}

	data, err := r.phase(ctx, PhaseDecodeGreedy, func() (any, error) {
		return MeasureDecode(ctx, r.Base, r.Model, r.Runs.Count("greedy"))
	})
	if err != nil {
		return nil, err
	}
	res.Decode = data.(DecodeResult)

	for _, d := range decodeSizes {
		key := fmt.Sprint(d.target)
		if !fitsContext(d.target, decodeMaxTokens, r.ContextLength) {
			reason := skipReason(d.target, decodeMaxTokens, r.ContextLength)
			res.Skipped[key] = reason
			res.DecodeAtContextSkipped[key] = reason
			r.emit(ctx, Event{Kind: EventSkipped, Phase: d.phase, Data: reason})
			continue
		}
		data, err := r.phase(ctx, d.phase, func() (any, error) {
			return MeasureDecodeAt(ctx, r.Base, r.Model, d.target, r.Runs.Count(d.key))
		})
		var sk skipError
		if errors.As(err, &sk) {
			res.Skipped[key] = sk.reason
			res.DecodeAtContextSkipped[key] = sk.reason
			continue
		}
		if err != nil {
			return nil, err
		}
		cd := data.(ContextDecode)
		res.DecodeAtContext[key] = cd.Decode
		res.Prefill[key] = cd.Prefill
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := WriteResult(res, r.Out)
	if err != nil {
		return nil, err
	}
	res.Path = path
	r.emit(ctx, Event{Kind: EventDone, Data: res})
	return res, nil
}

func DefaultOutPath(model string, t time.Time) string {
	return filepath.Join("results", fmt.Sprintf("%s-%s.json", t.UTC().Format("20060102T150405Z"), Slug(model)))
}

func WriteResult(res *Result, out string) (string, error) {
	if out == "" {
		out = DefaultOutPath(res.Model, res.StartedAt)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", err
	}
	buf, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return "", err
	}
	buf = append(buf, '\n')
	tmp := out + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, out); err != nil {
		return "", errors.Join(err, os.Remove(tmp))
	}
	return out, nil
}
