package bench

import (
	"strings"
	"testing"
)

func TestNonceDiffersBetweenCalls(t *testing.T) {
	a, err := newNonce()
	if err != nil {
		t.Fatal(err)
	}
	b, err := newNonce()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatalf("nonce repeated: %q", a)
	}
}

func TestPrefillMessagesPlacesNonceAfterSystemTag(t *testing.T) {
	msgs := prefillMessages("CTX", "session abc", prefillQuestion)
	if len(msgs) != 2 || msgs[0].Role != "system" || msgs[1].Role != "user" {
		t.Fatalf("unexpected message roles: %+v", msgs)
	}
	want := "[session abc] " + systemPrompt
	if !strings.HasPrefix(msgs[0].Content, want) {
		t.Fatalf("system content does not start with nonce: %q", msgs[0].Content[:40])
	}
	if !strings.HasSuffix(msgs[0].Content, "CTX") {
		t.Fatal("context missing from system message")
	}
	if msgs[1].Content != prefillQuestion {
		t.Fatal("question not used as user message")
	}
}

func TestDecodeContextMessagesNonceDiffers(t *testing.T) {
	a, err := decodeContextMessages("corpus text")
	if err != nil {
		t.Fatal(err)
	}
	b, err := decodeContextMessages("corpus text")
	if err != nil {
		t.Fatal(err)
	}
	if a[0].Content == b[0].Content {
		t.Fatal("decode system prompt must carry a fresh nonce on every call")
	}
	if !strings.Contains(a[0].Content, "# Repository context\ncorpus text") {
		t.Fatal("decode system prompt must carry the repository context")
	}
}

func TestDecodeMessagesRunTagDiffers(t *testing.T) {
	a, err := decodeMessages()
	if err != nil {
		t.Fatal(err)
	}
	b, err := decodeMessages()
	if err != nil {
		t.Fatal(err)
	}
	if a[1].Content == b[1].Content {
		t.Fatal("decode run tag repeated between calls")
	}
	if !strings.HasSuffix(a[1].Content, decodeUser) {
		t.Fatal("decode user prompt missing")
	}
}
