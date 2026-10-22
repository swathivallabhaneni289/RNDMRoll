package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// frozen is a clock that never moves: a test's buckets never refill, however
// slowly the machine runs it.
func frozen() time.Time { return time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC) }

func TestRateLimit_AllowsNThenRejectsTheNextWithinWindow(t *testing.T) {
	cfg := LimitConfig{
		Requests: 3,
		Window:   time.Second,
		KeyFunc:  KeyByIP,
		Clock:    frozen,
	}
	router := gin.New()
	router.GET("/ping", RateLimit(cfg), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < cfg.Requests; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 past the configured limit, got %d", w.Code)
	}
}

func TestRateLimit_DifferentKeysDoNotShareABucket(t *testing.T) {
	cfg := LimitConfig{
		Requests: 1,
		Window:   time.Second,
		KeyFunc: func(c *gin.Context) string {
			return c.GetHeader("X-Test-Key")
		},
		Clock: frozen,
	}
	router := gin.New()
	router.GET("/ping", RateLimit(cfg), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req1 := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req1.Header.Set("X-Test-Key", "key-a")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("key-a: expected 200, got %d", w1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req2.Header.Set("X-Test-Key", "key-b")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("key-b: expected 200 (independent bucket), got %d", w2.Code)
	}
}

// keyTestBody stands in for a handler's request type, binding rule and all.
type keyTestBody struct {
	Login    string `json:"login"`
	Password string `json:"password" binding:"required"`
}

func TestKeyByIPAndBody_ReadsTheNameAsTheHandlerDecodesIt(t *testing.T) {
	key := KeyByIPAndBody(func(b *keyTestBody) string { return b.Login })
	cases := []struct {
		name string
		body string
		want string
	}{
		{"the name", `{"login":"Some_Name","password":"x"}`, "192.0.2.1|some_name"},
		// Trimmed and lowercased as the handlers compare it, so padding is not
		// a new bucket.
		{"padded", `{"login":"\tSome_Name\n","password":"x"}`, "192.0.2.1|some_name"},
		// Keys match in any case and the last of two wins, as in the handler.
		{"key in capitals", `{"LOGIN":"victim","password":"x"}`, "192.0.2.1|victim"},
		{"decoy, then capitals", `{"login":"decoy","Login":"victim","password":"x"}`, "192.0.2.1|victim"},
		{"capitals, then decoy", `{"Login":"victim","login":"decoy","password":"x"}`, "192.0.2.1|decoy"},
		{"the same key twice", `{"login":"decoy","password":"x","login":"victim"}`, "192.0.2.1|victim"},
		{"empty, then capitals", `{"login":"","lOgIn":"victim","password":"x"}`, "192.0.2.1|victim"},
		// What follows the body is ignored, as the handler's decoder ignores it.
		{"a second body after", `{"login":"victim","password":"x"} {"login":"decoy"}`, "192.0.2.1|victim"},
		// The binding rules are the handler's: a body it will refuse still
		// counts against its name.
		{"no password", `{"login":"Some_Name"}`, "192.0.2.1|some_name"},
		{"no name", `{"password":"x"}`, "192.0.2.1|"},
		{"not json", `nope`, "192.0.2.1"},
		{"not text", `{"login":5,"password":"x"}`, "192.0.2.1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")

			if got := key(c); got != tc.want {
				t.Fatalf("key for %s: got %q, want %q", tc.body, got, tc.want)
			}
			// The handler still binds the body afterward and, when it accepts
			// it, reads the same name.
			var bound keyTestBody
			if err := c.ShouldBindBodyWith(&bound, binding.JSON); err == nil {
				if got := "192.0.2.1|" + strings.ToLower(strings.TrimSpace(bound.Login)); got != tc.want {
					t.Fatalf("the handler read %q, the key %q", got, tc.want)
				}
			}
		})
	}
}

// keyFor runs key on a request from remote carrying body.
func keyFor(t *testing.T, key func(*gin.Context) string, remote, body string) string {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	c.Request.RemoteAddr = remote
	c.Request.Header.Set("Content-Type", "application/json")
	return key(c)
}

// An IPv6 client is counted by its /64: walking through the addresses of the
// network it was given makes no new bucket, for the address limits and the
// name limits alike. IPv4 is counted address by address, also when written in
// IPv6 form.
func TestKeyByIP_CountsAnIPv6NetworkAsOneAddress(t *testing.T) {
	nameKey := KeyByIPAndBody(func(b *keyTestBody) string { return b.Login })
	cases := []struct {
		name, remote, want string
	}{
		{"IPv4", "198.51.100.7:4000", "198.51.100.7"},
		{"IPv4 in IPv6 form", "[::ffff:198.51.100.7]:4000", "198.51.100.7"},
		{"IPv6", "[2001:db8:1:2:aaaa:bbbb:cccc:dddd]:4000", "2001:db8:1:2::/64"},
		{"IPv6, same /64", "[2001:db8:1:2::1]:4000", "2001:db8:1:2::/64"},
		{"IPv6, next /64", "[2001:db8:1:3::1]:4000", "2001:db8:1:3::/64"},
		{"IPv6 loopback", "[::1]:4000", "::/64"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := keyFor(t, KeyByIP, tc.remote, ""); got != tc.want {
				t.Fatalf("KeyByIP from %s: got %q, want %q", tc.remote, got, tc.want)
			}
			if got := keyFor(t, nameKey, tc.remote, `{"login":"Victim"}`); got != tc.want+"|victim" {
				t.Fatalf("KeyByIPAndBody from %s: got %q, want %q", tc.remote, got, tc.want+"|victim")
			}
			if got := keyFor(t, nameKey, tc.remote, `nope`); got != tc.want {
				t.Fatalf("KeyByIPAndBody, no body, from %s: got %q, want %q", tc.remote, got, tc.want)
			}
		})
	}
	// A zone is not part of the key, and text that is not an address stays
	// as it is.
	if got := addressKey("fe80::1:2:3:4%en0"); got != "fe80::/64" {
		t.Fatalf("zoned address: %q", got)
	}
	if got := addressKey(""); got != "" {
		t.Fatalf("no address: %q", got)
	}
}

// Two IPv6 addresses of one /64 share one bucket of the per-address limit.
func TestRateLimit_AnIPv6NetworkSharesOneBucket(t *testing.T) {
	router := gin.New()
	router.GET("/ping", RateLimit(LimitConfig{Requests: 2, Window: time.Minute, KeyFunc: KeyByIP, Clock: frozen}), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	send := func(remote string) int {
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = remote
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}
	if send("[2001:db8:9:9::1]:1") != http.StatusOK || send("[2001:db8:9:9::2]:1") != http.StatusOK {
		t.Fatal("the first two requests from the network were refused")
	}
	if code := send("[2001:db8:9:9:ffff::3]:1"); code != http.StatusTooManyRequests {
		t.Fatalf("a third address in the same /64: %d, want 429", code)
	}
	if code := send("[2001:db8:9:a::1]:1"); code != http.StatusOK {
		t.Fatalf("another /64: %d, want 200", code)
	}
}

// A name over 256 bytes is keyed by its hash: the key stays small, a
// different long name is a different bucket, and padding or capitals are
// still not.
func TestKeyByIPAndBody_HashesALongName(t *testing.T) {
	key := KeyByIPAndBody(func(b *keyTestBody) string { return b.Login })
	long := strings.Repeat("a", 300) + "@example.com"
	got := keyFor(t, key, "192.0.2.1:1", `{"login":"`+long+`"}`)
	if len(got) > 100 || !strings.HasPrefix(got, "192.0.2.1|sha256:") {
		t.Fatalf("key for a long name: %q (%d bytes)", got, len(got))
	}
	if again := keyFor(t, key, "192.0.2.1:1", `{"login":"  `+strings.ToUpper(long)+` "}`); again != got {
		t.Fatalf("padding and capitals made a new bucket: %q, %q", again, got)
	}
	if other := keyFor(t, key, "192.0.2.1:1", `{"login":"b`+long+`"}`); other == got {
		t.Fatal("two long names share a bucket")
	}
	exact := strings.Repeat("a", 256)
	if got := keyFor(t, key, "192.0.2.1:1", `{"login":"`+exact+`"}`); got != "192.0.2.1|"+exact {
		t.Fatalf("a name of exactly 256 bytes is kept as written, got %q", got)
	}
}

// fakeClock is a clock the test moves.
type fakeClock struct{ now time.Time }

func (f *fakeClock) Now() time.Time { return f.now }

// has reports whether l holds a bucket for key.
func (l *Limiter) has(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, ok := l.entries[key]
	return ok
}

// size is how many keys l holds.
func (l *Limiter) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

// A key idle for more than ten windows is dropped at the next request (its
// bucket has long been full again); a key seen since is kept, and a request
// walks only the keys it drops.
func TestLimiter_DropsKeysIdleForTenWindows(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)}
	start := clock.now
	l := NewLimiter(5, time.Minute, clock.Now)

	l.Allow("a")
	clock.now = start.Add(5 * time.Minute)
	l.Allow("b")
	// Ten windows after "a" was seen: idle exactly ten windows, so it stays.
	clock.now = start.Add(10 * time.Minute)
	l.Allow("c")
	if !l.has("a") || !l.has("b") {
		t.Fatal("a key idle for ten windows or less was dropped")
	}
	clock.now = start.Add(10*time.Minute + time.Second)
	l.Allow("c")
	if l.has("a") {
		t.Fatal("a key idle for more than ten windows survived the next request")
	}
	if !l.has("b") || !l.has("c") {
		t.Fatal("keys still in use were dropped")
	}
	// "b", seen again, is kept past its first ten windows.
	clock.now = start.Add(14 * time.Minute)
	l.Allow("b")
	clock.now = start.Add(16 * time.Minute)
	l.Allow("d")
	if !l.has("b") {
		t.Fatal("a key seen again was dropped by its first sighting's age")
	}
}

// The map never holds more than maxKeys: a new key in a full map drops the
// key seen longest ago, and seeing a key again makes it the newest.
func TestLimiter_KeepsAtMostMaxKeysDroppingTheOneSeenLongestAgo(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)}
	l := NewLimiter(5, time.Minute, clock.Now)
	l.maxKeys = 3
	for _, k := range []string{"a", "b", "c"} {
		l.Allow(k)
		clock.now = clock.now.Add(time.Second)
	}
	l.Allow("a") // "b" is now the one seen longest ago
	l.Allow("d")
	if l.size() != 3 || l.has("b") || !l.has("a") || !l.has("c") || !l.has("d") {
		t.Fatalf("after d: %d keys, a %v b %v c %v d %v; want a, c, d", l.size(), l.has("a"), l.has("b"), l.has("c"), l.has("d"))
	}
	for i := range 100 {
		l.Allow(fmt.Sprintf("other-%d", i))
	}
	if l.size() != 3 {
		t.Fatalf("%d keys, want at most 3", l.size())
	}
	// The default bound is the package's.
	if NewLimiter(5, time.Minute, nil).maxKeys != maxLimiterKeys {
		t.Fatal("a new Limiter does not keep maxLimiterKeys")
	}
}

// An IPv6 /64 is also counted under its /48, at ten times the rate: walking
// through the /64s of one /48 stops at ten times one /64's share, for the
// address limits and the name limits alike. Another /48 has its own.
func TestLimiter_CountsAnIPv6NetworkUnderIts48Too(t *testing.T) {
	for _, name := range []string{"", "|victim"} {
		l := NewLimiter(2, time.Minute, frozen)
		key := func(n int) string { return fmt.Sprintf("2001:db8:7:%x::/64", n) + name }
		for n := range 10 {
			if !l.Allow(key(n)) || !l.Allow(key(n)) {
				t.Fatalf("%q: /64 %d was refused within its share", name, n)
			}
			if l.Allow(key(n)) {
				t.Fatalf("%q: /64 %d got a third", name, n)
			}
		}
		if l.Allow(key(10)) {
			t.Fatalf("%q: an 11th /64 of the /48 got a request past the /48's 20", name)
		}
		if !l.Allow("2001:db8:8::/64" + name) {
			t.Fatalf("%q: another /48 was refused", name)
		}
		if !l.Allow("198.51.100.7"+name) || !l.Allow("198.51.100.7"+name) {
			t.Fatalf("%q: an IPv4 address was refused within its share", name)
		}
	}
}

// A request one bucket refuses is taken from neither: a /64 at its own limit
// does not use up its /48's share, and a /64 turned away by its /48 keeps its
// own share for when the /48 has room again.
func TestLimiter_ARefusedRequestCostsNothing(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)}
	l := NewLimiter(1, time.Minute, clock.Now)
	key := func(n int) string { return fmt.Sprintf("2001:db8:9:%x::/64", n) }

	if !l.Allow(key(0)) {
		t.Fatal("the first request was refused")
	}
	for range 50 {
		if l.Allow(key(0)) {
			t.Fatal("a /64 went past its own share")
		}
	}
	// The /48's other nine requests are all still there.
	for n := 1; n <= 9; n++ {
		if !l.Allow(key(n)) {
			t.Fatalf("/64 %d was refused: the refused requests used up the /48", n)
		}
	}
	if l.Allow(key(10)) {
		t.Fatal("an 11th request went past the /48's share")
	}
	// Six seconds on the /48 has one request again (10 a minute), while a
	// /64's own share needs a minute. The /64 turned away above still has
	// its one.
	clock.now = clock.now.Add(6 * time.Second)
	if !l.Allow(key(10)) {
		t.Fatal("the /64 the /48 turned away lost its own share")
	}
}

// wideKey swaps a leading /64 for its /48 and keeps the rest; any other key
// has no /48.
func TestWideKey(t *testing.T) {
	cases := map[string]string{
		"2001:db8:1:2::/64":            "2001:db8:1::/48",
		"2001:db8:1:2::/64|victim":     "2001:db8:1::/48|victim",
		"2001:db8:1:2::/64|a|b":        "2001:db8:1::/48|a|b",
		"::/64":                        "::/48",
		"198.51.100.7":                 "",
		"198.51.100.7|victim":          "",
		"fe80::1%en0":                  "",
		"2001:db8::/48":                "",
		"1c0d1a6e-0000-4000-8000-1234": "",
		"":                             "",
	}
	for key, want := range cases {
		if got := wideKey(key); got != want {
			t.Errorf("wideKey(%q) = %q, want %q", key, got, want)
		}
	}
}

// A client whose address gin cannot read (a link-local address with a zone,
// or an address with no port) is keyed by its host, never by one empty key
// that every such client would share. A zone is kept: every link-local
// address is in fe80::/64.
func TestKeyByIP_AnAddressGinCannotReadIsKeyedByItsHost(t *testing.T) {
	cases := []struct{ remote, want string }{
		{"[fe80::1%en0]:4000", "fe80::1%en0"},
		{"[fe80::2%en0]:4000", "fe80::2%en0"},
		{"[FE80::1%en1]:4000", "fe80::1%en1"},
		{"198.51.100.7", "198.51.100.7"},
		{"2001:db8:1:2::5", "2001:db8:1:2::/64"},
		{"not an address", "not an address"},
	}
	for _, tc := range cases {
		if got := keyFor(t, KeyByIP, tc.remote, ""); got != tc.want {
			t.Errorf("KeyByIP from %q: got %q, want %q", tc.remote, got, tc.want)
		}
	}

	router := gin.New()
	router.GET("/ping", RateLimit(LimitConfig{Requests: 1, Window: time.Minute, KeyFunc: KeyByIP, Clock: frozen}), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	send := func(remote string) int {
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = remote
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}
	if send("[fe80::1%en0]:1") != http.StatusOK || send("[fe80::1%en0]:2") != http.StatusTooManyRequests {
		t.Fatal("one link-local client is not limited")
	}
	if code := send("[fe80::2%en0]:1"); code != http.StatusOK {
		t.Fatalf("another link-local client shares the first one's bucket: %d", code)
	}
}

// RateLimit counts on the configured clock: nothing refills until it moves.
func TestRateLimit_CountsOnItsClock(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)}
	router := gin.New()
	router.GET("/ping", RateLimit(LimitConfig{Requests: 1, Window: time.Minute, KeyFunc: KeyByIP, Clock: clock.Now}), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	send := func() int {
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}
	if send() != http.StatusOK || send() != http.StatusTooManyRequests {
		t.Fatal("the first request was refused or the second allowed")
	}
	clock.now = clock.now.Add(59 * time.Second)
	if send() != http.StatusTooManyRequests {
		t.Fatal("a request came back before its minute")
	}
	clock.now = clock.now.Add(time.Second)
	if send() != http.StatusOK {
		t.Fatal("the minute passed on the clock, and the request was still refused")
	}
}

// The buckets fill and refill on the Limiter's own clock.
func TestLimiter_RefillsOnItsClock(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)}
	l := NewLimiter(2, time.Minute, clock.Now)
	if !l.Allow("k") || !l.Allow("k") {
		t.Fatal("the first two were refused")
	}
	if l.Allow("k") {
		t.Fatal("a third within the minute was allowed")
	}
	if !l.Allow("other") {
		t.Fatal("another key shares the bucket")
	}
	clock.now = clock.now.Add(30 * time.Second)
	if !l.Allow("k") {
		t.Fatal("half a minute on, one more was refused")
	}
	if l.Allow("k") {
		t.Fatal("half a minute on, two more were allowed")
	}
}

// A /48 is asked before its /64's bucket is made: the /64s a /48 turns away
// add no key, so a flood of fresh /64s from one /48 stores no more keys than
// the /48 lets through, and cannot push out another key's used-up bucket (an
// IPv4 address's, or another /48's) to hand it a full one.
func TestLimiter_AFloodFromOne48CannotPushOutAUsedUpBucket(t *testing.T) {
	l := NewLimiter(1, time.Minute, frozen)
	l.maxKeys = 100
	const used = "198.51.100.7"
	if !l.Allow(used) || l.Allow(used) {
		t.Fatal("the IPv4 address was refused its one request or allowed a second")
	}
	// Another /48 uses up its share: ten requests, from ten /64s.
	other := func(n int) string { return fmt.Sprintf("2001:db8:aa:%x::/64", n) }
	for n := range 10 {
		if !l.Allow(other(n)) {
			t.Fatalf("/64 %d of the other /48 was refused within its share", n)
		}
	}
	if l.Allow(other(10)) {
		t.Fatal("the other /48 went past its share")
	}
	// One /48 sends a thousand requests, each from a fresh /64.
	flood := func(n int) string { return fmt.Sprintf("2001:db8:bb:%x::/64", n) }
	allowed := 0
	for n := range 1000 {
		if l.Allow(flood(n)) {
			allowed++
		}
	}
	if allowed != 10 {
		t.Fatalf("the flooding /48 got %d requests, want its share of 10", allowed)
	}
	// Kept: the address, the two /48s and the ten /64s each let through.
	if got, want := l.size(), 1+2+10+10; got != want {
		t.Fatalf("%d keys after the flood, want %d", got, want)
	}
	if l.has(other(10)) || l.has(flood(500)) {
		t.Fatal("a /64 its /48 turned away was stored")
	}
	if l.Allow(used) {
		t.Fatal("the flood pushed out the used-up IPv4 bucket")
	}
	if l.Allow(other(11)) {
		t.Fatal("the flood pushed out the other /48's used-up bucket")
	}
}
