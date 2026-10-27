package bench

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"sync"
)

//go:embed corpus/bench_corpus.txt
var corpusText string

const (
	CorpusSHA256    = "58a7756fd94fc48a7b7306d330b2c44663bdf22faa943f82430966c2878081b2"
	CharsPerToken   = 4.25
	WarmTokens      = 2700
	WarmOffsetChars = 600000
)

var (
	corpusOnce  sync.Once
	corpusRunes []rune
)

func corpus() []rune {
	corpusOnce.Do(func() { corpusRunes = []rune(corpusText) })
	return corpusRunes
}

func checkCorpus(text string) error {
	sum := sha256.Sum256([]byte(text))
	if got := hex.EncodeToString(sum[:]); got != CorpusSHA256 {
		return fmt.Errorf("prompt corpus sha256 mismatch: got %s, want %s", got, CorpusSHA256)
	}
	return nil
}

func VerifyCorpus() error {
	return checkCorpus(corpusText)
}

func BuildContext(tokens int) (string, error) {
	return BuildContextAt(tokens, 0)
}

func BuildContextAt(tokens, offset int) (string, error) {
	if tokens <= 0 || offset < 0 {
		return "", fmt.Errorf("invalid context request: tokens=%d offset=%d", tokens, offset)
	}
	chars := corpus()
	end := offset + int(float64(tokens)*CharsPerToken)
	if end > len(chars) {
		return "", fmt.Errorf("a %d-token prompt at offset %d needs %d characters, corpus has %d", tokens, offset, end, len(chars))
	}
	return string(chars[offset:end]), nil
}
