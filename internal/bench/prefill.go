package bench

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
)

const (
	prefillMaxTokens = 256
	coldCacheMax     = 16
	promptTolerance  = 0.01
)

var referencePromptTokens = map[int]int{4000: 4133, 16000: 16286}

func Warmup(ctx context.Context, base, model string) error {
	ctxText, err := BuildContextAt(WarmTokens, WarmOffsetChars)
	if err != nil {
		return err
	}
	nonce, err := newNonce()
	if err != nil {
		return err
	}
	body := greedyBody(model, prefillMessages(ctxText, nonce, prefillQuestion), prefillMaxTokens)
	_, err = StreamChat(ctx, base, body)
	return err
}

func MeasurePrefill(ctx context.Context, base, model string, target int) (PrefillResult, error) {
	res := PrefillResult{Target: target}
	ctxText, err := BuildContext(target)
	if err != nil {
		return res, err
	}
	nonce, err := newNonce()
	if err != nil {
		return res, err
	}
	body := greedyBody(model, prefillMessages(ctxText, nonce, prefillQuestion), prefillMaxTokens)
	stream, err := StreamChat(ctx, base, body)
	if err != nil {
		return res, fmt.Errorf("prefill %d: %w", target, err)
	}
	return validatePrefill(model, target, stream), nil
}

func prefillFromRuns(model string, target int, streams []*StreamResult) PrefillResult {
	res := PrefillResult{Target: target}
	for _, s := range streams {
		run := validatePrefill(model, target, s)
		res.TTFTSamples = append(res.TTFTSamples, run.TTFTSeconds)
		if res.PromptTokens == 0 {
			res.PromptTokens = run.PromptTokens
		}
		if run.CachedTokens > res.CachedTokens {
			res.CachedTokens = run.CachedTokens
		}
		res.Invalid = appendUnique(res.Invalid, run.Invalid...)
		res.Flags = appendUnique(res.Flags, run.Flags...)
	}
	res.TTFTSeconds = Median(res.TTFTSamples)
	if res.PromptTokens > 0 && res.TTFTSeconds > 0 {
		res.TPS = float64(res.PromptTokens) / res.TTFTSeconds
	}
	return res
}

func appendUnique(dst []string, src ...string) []string {
	for _, s := range src {
		if !slices.Contains(dst, s) {
			dst = append(dst, s)
		}
	}
	return dst
}

func validatePrefill(model string, target int, s *StreamResult) PrefillResult {
	res := PrefillResult{Target: target}
	res.TTFTSeconds = s.TTFT.Seconds()
	res.CachedTokens = s.Usage.Cached()
	if s.Usage == nil || s.Usage.PromptTokens <= 0 {
		res.Invalid = append(res.Invalid, "usage.prompt_tokens missing")
	} else {
		res.PromptTokens = s.Usage.PromptTokens
	}
	if s.TTFT <= 0 {
		res.Invalid = append(res.Invalid, "no content-bearing stream event")
	}
	if res.PromptTokens > 0 && res.TTFTSeconds > 0 {
		res.TPS = float64(res.PromptTokens) / res.TTFTSeconds
	}
	if res.CachedTokens > coldCacheMax {
		res.Invalid = append(res.Invalid, fmt.Sprintf("cached_tokens %d > %d on a cold prompt", res.CachedTokens, coldCacheMax))
	}
	ref, hasRef := referencePromptTokens[target]
	switch {
	case !hasRef:
		res.Flags = append(res.Flags, "prompt_tokens unchecked (no reference count for this size)")
	case !isQwen(model):
		res.Flags = append(res.Flags, "prompt_tokens unchecked (non-Qwen model)")
	case res.PromptTokens > 0 && math.Abs(float64(res.PromptTokens-ref))/float64(ref) > promptTolerance:
		res.Invalid = append(res.Invalid, fmt.Sprintf("prompt_tokens %d outside ±1%% of reference %d", res.PromptTokens, ref))
	}
	return res
}

func isQwen(model string) bool {
	return strings.Contains(strings.ToLower(model), "qwen")
}
