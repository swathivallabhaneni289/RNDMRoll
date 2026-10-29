package adminlink

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var (
	secret = []byte("adminlink-test-secret-at-least-32-bytes")
	now    = time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)
)

func TestNewAndRead(t *testing.T) {
	for _, email := range []string{"", "boss.admin+1@example.com"} {
		token, err := New(secret, email, now)
		if err != nil {
			t.Fatal(err)
		}
		l, err := Read(secret, token, now.Add(TTL-time.Second))
		if err != nil {
			t.Fatalf("%q: %v", email, err)
		}
		if l.Email != email || l.Nonce == "" || !l.Expires.Equal(now.Add(TTL)) {
			t.Fatalf("%q: read %+v", email, l)
		}
	}
	a, _ := New(secret, "", now)
	b, _ := New(secret, "", now)
	la, _ := Read(secret, a, now)
	lb, _ := Read(secret, b, now)
	if la.Nonce == lb.Nonce {
		t.Fatal("two links share a nonce")
	}
}

func TestReadRefuses(t *testing.T) {
	token, err := New(secret, "boss@example.com", now)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	other, _ := New([]byte("another-secret-that-is-at-least-32-bytes"), "boss@example.com", now)
	cases := map[string]struct {
		token string
		at    time.Time
	}{
		"at its end":            {token, now.Add(TTL)},
		"after its end":         {token, now.Add(time.Hour)},
		"made too far ahead":    {token, now.Add(-time.Minute - time.Second)},
		"another secret":        {other, now},
		"another email":         {"Zm9vQGV4YW1wbGUuY29t." + parts[1] + "." + parts[2] + "." + parts[3], now},
		"a later end":           {parts[0] + "." + "9999999999" + "." + parts[2] + "." + parts[3], now},
		"no signature":          {parts[0] + "." + parts[1] + "." + parts[2], now},
		"no nonce":              {parts[0] + "." + parts[1] + ".." + parts[3], now},
		"an extra part":         {token + ".x", now},
		"empty":                 {"", now},
		"the admin cookie kind": {parts[0] + "." + parts[1] + "." + parts[3], now},
	}
	for name, c := range cases {
		if _, err := Read(secret, c.token, c.at); !errors.Is(err, ErrBad) {
			t.Errorf("%s: %v, want ErrBad", name, err)
		}
	}
	if _, err := Read(secret, token, now.Add(-time.Minute)); err != nil {
		t.Errorf("a minute of clock difference: %v", err)
	}
}

func TestNewNeedsALongSecret(t *testing.T) {
	if _, err := New([]byte("short"), "", now); err == nil {
		t.Fatal("a short secret signed a link")
	}
}
