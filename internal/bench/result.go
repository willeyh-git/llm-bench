package bench

import "sort"

type PrefillResult struct {
	Target       int      `json:"target"`
	PromptTokens int      `json:"prompt_tokens"`
	TTFTSeconds  float64  `json:"ttft_seconds"`
	TPS          float64  `json:"tps"`
	CachedTokens int      `json:"cached_tokens"`
	Invalid      []string `json:"invalid,omitempty"`
	Flags        []string `json:"flags,omitempty"`
}

type DecodeRun struct {
	Tokens       int      `json:"tokens"`
	FinishReason string   `json:"finish_reason"`
	TPSWindow    float64  `json:"tps_window"`
	TPSEndToEnd  float64  `json:"tps_end_to_end"`
	Flags        []string `json:"flags,omitempty"`
}

type DecodeResult struct {
	Runs      []DecodeRun `json:"runs"`
	TPSMedian float64     `json:"tps_median"`
	TPSMin    float64     `json:"tps_min"`
	TPSMax    float64     `json:"tps_max"`
	Invalid   []string    `json:"invalid,omitempty"`
	Flags     []string    `json:"flags,omitempty"`
}

func Median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	s := append([]float64(nil), vals...)
	sort.Float64s(s)
	mid := len(s) / 2
	if len(s)%2 == 1 {
		return s[mid]
	}
	return (s[mid-1] + s[mid]) / 2
}

func Min(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	lo := vals[0]
	for _, v := range vals[1:] {
		if v < lo {
			lo = v
		}
	}
	return lo
}

func Max(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	hi := vals[0]
	for _, v := range vals[1:] {
		if v > hi {
			hi = v
		}
	}
	return hi
}
