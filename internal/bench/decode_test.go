package bench

import (
	"math"
	"testing"
	"time"
)

func fixtureEvents(n int, step time.Duration) []time.Duration {
	ev := make([]time.Duration, n)
	for i := range ev {
		ev[i] = time.Duration(i+1) * step
	}
	return ev
}

func TestWindowRateDropsFirst16AndLast4(t *testing.T) {
	ev := fixtureEvents(512, 10*time.Millisecond)
	got := windowRate(ev)
	window := ev[16 : 512-4]
	want := float64(len(window)-1) / (window[len(window)-1] - window[0]).Seconds()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("windowRate = %v, want %v", got, want)
	}
	if math.Abs(got-100) > 1e-6 {
		t.Fatalf("uniform 100 tok/s stream gave %v", got)
	}
}

func TestWindowRateTooShort(t *testing.T) {
	if got := windowRate(fixtureEvents(10, time.Millisecond)); got != 0 {
		t.Fatalf("short stream rate = %v, want 0", got)
	}
}

func TestEndToEndRate(t *testing.T) {
	s := &StreamResult{Total: 2 * time.Second}
	if got := endToEndRate(512, s); math.Abs(got-511/2.0) > 1e-9 {
		t.Fatalf("end-to-end = %v", got)
	}
}

func TestMedianMinMax(t *testing.T) {
	vals := []float64{30, 10, 50, 20, 40}
	if got := Median(vals); got != 30 {
		t.Errorf("median = %v, want 30", got)
	}
	if got := Min(vals); got != 10 {
		t.Errorf("min = %v, want 10", got)
	}
	if got := Max(vals); got != 50 {
		t.Errorf("max = %v, want 50", got)
	}
	if got := Median([]float64{1, 2, 3, 4}); got != 2.5 {
		t.Errorf("even median = %v, want 2.5", got)
	}
	if Median(nil) != 0 || Min(nil) != 0 || Max(nil) != 0 {
		t.Error("empty input should return 0")
	}
}

func TestDecodeRunFlagsBadFinishAndCount(t *testing.T) {
	s := &StreamResult{
		Finish:  "stop",
		Usage:   &Usage{CompletionTokens: 300},
		EventAt: fixtureEvents(300, 10*time.Millisecond),
		Total:   3 * time.Second,
	}
	run := decodeRun(s)
	if len(run.Flags) != 2 {
		t.Fatalf("flags = %v, want 2 entries", run.Flags)
	}
	if run.Tokens != 300 {
		t.Errorf("tokens = %d, the number must be kept", run.Tokens)
	}
	if run.TPSWindow <= 0 {
		t.Error("flagged run should still report a rate")
	}
}

func TestDecodeRunCleanHasNoFlags(t *testing.T) {
	s := &StreamResult{
		Finish:  "length",
		Usage:   &Usage{CompletionTokens: 512},
		EventAt: fixtureEvents(512, 10*time.Millisecond),
		Total:   6 * time.Second,
	}
	if run := decodeRun(s); len(run.Flags) != 0 {
		t.Fatalf("unexpected flags: %v", run.Flags)
	}
}

func TestMeasureDecodeFiveRunsAgainstFakeServer(t *testing.T) {
	srv := sseServer(t, decodeStream())
	defer srv.Close()

	res, err := MeasureDecode(t.Context(), srv.URL+"/v1", "Gemma-4-12B")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Runs) != decodeRuns {
		t.Fatalf("runs = %d, want %d", len(res.Runs), decodeRuns)
	}
	if res.TPSMin > res.TPSMedian || res.TPSMedian > res.TPSMax {
		t.Errorf("order broken: min %v median %v max %v", res.TPSMin, res.TPSMedian, res.TPSMax)
	}
}

func decodeStream() string {
	s := ""
	for i := 0; i < 3; i++ {
		s += "data: {\"choices\":[{\"delta\":{\"content\":\"x\"},\"finish_reason\":null}]}\n\n"
	}
	s += "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"length\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":3}}\n\n"
	s += "data: [DONE]\n\n"
	return s
}
