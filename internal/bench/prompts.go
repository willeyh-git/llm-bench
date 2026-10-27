package bench

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

const (
	systemPrompt = "You are Nace, a local coding agent. You help the user with their repository. " +
		"Be precise. Use tools when you need to read files or run commands."
	decodeSystem = "You are a helpful coding assistant. Answer with code only."
	decodeUser   = "Write a complete, well-commented Python module that implements an LRU cache with TTL expiry, " +
		"a thread-safe wrapper, and a pytest test suite. Write at least 600 lines. Do not stop early."
	prefillQuestion = "In one sentence, what is the first file in the repository context about?"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func newNonce() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("session %016x", binary.BigEndian.Uint64(b[:])), nil
}

func prefillMessages(ctx, nonce, question string) []Message {
	return []Message{
		{Role: "system", Content: fmt.Sprintf("[%s] %s\n\n# Repository context\n%s", nonce, systemPrompt, ctx)},
		{Role: "user", Content: question},
	}
}

func decodeMessages() ([]Message, error) {
	tag, err := newNonce()
	if err != nil {
		return nil, err
	}
	return []Message{
		{Role: "system", Content: decodeSystem},
		{Role: "user", Content: fmt.Sprintf("[run %s] %s", tag, decodeUser)},
	}, nil
}

func decodeContextMessages(ctxText string) ([]Message, error) {
	tag, err := newNonce()
	if err != nil {
		return nil, err
	}
	return []Message{
		{Role: "system", Content: fmt.Sprintf("[%s] %s\n\n# Repository context\n%s", tag, decodeSystem, ctxText)},
		{Role: "user", Content: decodeUser},
	}, nil
}
