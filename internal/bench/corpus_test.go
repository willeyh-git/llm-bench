package bench

import (
	"testing"
)

func TestCorpusChecksum(t *testing.T) {
	if err := VerifyCorpus(); err != nil {
		t.Fatal(err)
	}
}

func TestCheckCorpusDetectsModification(t *testing.T) {
	if err := checkCorpus(corpusText + "x"); err == nil {
		t.Fatal("expected checksum mismatch for modified corpus")
	}
}

func TestBuildContextSizes(t *testing.T) {
	cases := []struct {
		tokens    int
		wantChars int
	}{
		{4000, 17000},
		{16000, 68000},
	}
	for _, c := range cases {
		ctx, err := BuildContext(c.tokens)
		if err != nil {
			t.Fatal(err)
		}
		if n := len([]rune(ctx)); n != c.wantChars {
			t.Errorf("BuildContext(%d) = %d chars, want %d", c.tokens, n, c.wantChars)
		}
	}
}

func TestBuildContextAtOffsetIsSlice(t *testing.T) {
	ctx, err := BuildContextAt(WarmTokens, WarmOffsetChars)
	if err != nil {
		t.Fatal(err)
	}
	want := string(corpus()[WarmOffsetChars : WarmOffsetChars+int(float64(WarmTokens)*CharsPerToken)])
	if ctx != want {
		t.Fatal("warm-up context is not the corpus slice at its offset")
	}
}

func TestBuildContextRejectsTooLarge(t *testing.T) {
	if _, err := BuildContextAt(1_000_000, 0); err == nil {
		t.Fatal("expected error for a prompt longer than the corpus")
	}
}

func TestBuildContextIsDeterministic(t *testing.T) {
	a, _ := BuildContext(4000)
	b, _ := BuildContext(4000)
	if a != b {
		t.Fatal("same request produced different text")
	}
}
