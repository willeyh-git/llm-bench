package bench

import (
	"context"
	"fmt"
)

const (
	decodeMaxTokens = 512
	decodeRuns      = 5
)

func decodeRate(tokens int, s *StreamResult) float64 {
	if tokens < 2 || s == nil {
		return 0
	}
	ttft := s.TTFT
	if ttft <= 0 && len(s.EventAt) > 0 {
		ttft = s.EventAt[0]
	}
	span := s.Total - ttft
	if span <= 0 {
		return 0
	}
	return float64(tokens-1) / span.Seconds()
}

func endToEndRate(tokens int, s *StreamResult) float64 {
	if tokens < 2 || s.Total <= 0 {
		return 0
	}
	return float64(tokens-1) / s.Total.Seconds()
}

type ContextDecode struct {
	Decode  DecodeResult
	Prefill PrefillResult
}

func decodeRun(s *StreamResult) DecodeRun {
	run := DecodeRun{FinishReason: s.Finish, TTFTSeconds: s.TTFT.Seconds()}
	if s.Usage != nil {
		run.Tokens = s.Usage.CompletionTokens
		run.PromptTokens = s.Usage.PromptTokens
	} else {
		run.Tokens = len(s.EventAt)
	}
	run.TPSWindow = decodeRate(run.Tokens, s)
	run.TPSEndToEnd = endToEndRate(run.Tokens, s)
	if s.Finish != "length" {
		run.Flags = append(run.Flags, fmt.Sprintf("finish_reason %q, expected \"length\"", s.Finish))
	}
	if run.Tokens != decodeMaxTokens {
		run.Flags = append(run.Flags, fmt.Sprintf("completion_tokens %d, expected %d", run.Tokens, decodeMaxTokens))
	}
	return run
}

func MeasureDecode(ctx context.Context, base, model string, runs int) (DecodeResult, error) {
	res, _, err := measureDecode(ctx, base, model, runs, decodeMessages)
	return res, err
}

func MeasureDecodeAt(ctx context.Context, base, model string, target, runs int) (ContextDecode, error) {
	ctxText, err := BuildContext(target)
	if err != nil {
		return ContextDecode{}, err
	}
	res, streams, err := measureDecode(ctx, base, model, runs, func() ([]Message, error) {
		return decodeContextMessages(ctxText)
	})
	if err != nil {
		return ContextDecode{}, err
	}
	return ContextDecode{Decode: res, Prefill: prefillFromRuns(model, target, streams)}, nil
}

func measureDecode(ctx context.Context, base, model string, runs int, build func() ([]Message, error)) (DecodeResult, []*StreamResult, error) {
	var res DecodeResult
	var streams []*StreamResult
	for i := 0; i < runs; i++ {
		msgs, err := build()
		if err != nil {
			return res, nil, err
		}
		stream, err := StreamChat(ctx, base, greedyBody(model, msgs, decodeMaxTokens))
		if err != nil {
			return res, nil, fmt.Errorf("decode run %d: %w", i+1, err)
		}
		if stream.Reasoning != "" {
			res.Invalid = append(res.Invalid, "reasoning output present, thinking was not disabled")
		}
		streams = append(streams, stream)
		res.Runs = append(res.Runs, decodeRun(stream))
	}
	rates := make([]float64, len(res.Runs))
	for i, r := range res.Runs {
		rates[i] = r.TPSWindow
	}
	res.TPSMedian = Median(rates)
	res.TPSMin = Min(rates)
	res.TPSMax = Max(rates)
	return res, streams, nil
}
