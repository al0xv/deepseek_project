package crypto

import "testing"

func TestNewTokenLength(t *testing.T) {
	tok, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(tok) != 43 {
		t.Fatalf("want 43 chars (32 bytes base64url), got %d: %q", len(tok), tok)
	}
}

func TestNewPairCodeFormat(t *testing.T) {
	for i := 0; i < 200; i++ {
		code, err := NewPairCode()
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != 6 {
			t.Fatalf("want 6 digits, got %q", code)
		}
		for _, c := range code {
			if c < '0' || c > '9' {
				t.Fatalf("non-digit in code %q", code)
			}
		}
	}
}

func TestTokensUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		tok, err := NewToken()
		if err != nil {
			t.Fatal(err)
		}
		if seen[tok] {
			t.Fatal("duplicate token generated")
		}
		seen[tok] = true
	}
}

func TestNewIDFormat(t *testing.T) {
	id, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 22 {
		t.Fatalf("want 22 chars (16 bytes base64url), got %d", len(id))
	}
}
