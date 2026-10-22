package middleware

import (
	"bytes"
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/gin-gonic/gin/codec/json"
	"golang.org/x/time/rate"
)

// LimitConfig configures a token-bucket rate limit: Requests allowed per
// Window, keyed by KeyFunc. Clock is the time the buckets fill and refill by:
// nil is the real clock, and a test passes one it moves, so a limit never
// depends on how fast the machine runs the requests.
type LimitConfig struct {
	Requests int
	Window   time.Duration
	KeyFunc  func(*gin.Context) string
	Clock    func() time.Time
}

const (
	// maxLimiterKeys is the most keys one Limiter keeps, so no source of
	// fresh keys can grow its memory without bound: measured, a full one
	// holds about 15 MB (25 MB when every key carries a 256-byte name). When
	// it is full, the key seen longest ago is dropped. The bound is each
	// Limiter's, not the process's.
	maxLimiterKeys = 50_000
	// wideFactor is how many times the requests of one IPv6 /64 the /48 it is
	// in may make, in the same window (wideKey).
	wideFactor = 10
)

type limiterEntry struct {
	key      string
	limiter  *rate.Limiter
	lastSeen time.Time
}

// Limiter allows Requests a Window for each key: one *rate.Limiter per key,
// held in a mutex-guarded map, with the keys in the order they were last
// seen. A key idle for ten windows is dropped (its bucket has long been full
// again, so forgetting it changes nothing), and the map never holds more than
// maxLimiterKeys: past that, the key seen longest ago goes. Either way only
// the keys dropped are looked at, so no request ever walks the map. Dropping
// a key seen within its last window gives it a full bucket again; that is the
// price of bounded memory, met only when that many fresh keys come within a
// window.
//
// A key that starts with an IPv6 /64 (KeyByIP) is also counted under the /48
// that holds it, at ten times the rate (wideKey): a /48 holds 65,536 /64s,
// so counting only the /64 would let one client walk through them for a fresh
// bucket each time. The /48 is asked first, so the /64s it turns away are
// never stored (Allow).
//
// RateLimit is a Limiter in front of a route; a handler can hold one of its
// own to count a key only it knows (Log in counts the account it found).
type Limiter struct {
	mu         sync.Mutex
	entries    map[string]*list.Element // each holds a *limiterEntry
	recent     *list.List               // the entries, the one seen last first
	limit      rate.Limit
	burst      int
	evictAfter time.Duration
	maxKeys    int
	now        func() time.Time
}

// NewLimiter builds a Limiter allowing requests per window for each key, on
// clock (nil is the real clock).
func NewLimiter(requests int, window time.Duration, clock func() time.Time) *Limiter {
	if clock == nil {
		clock = time.Now
	}
	return &Limiter{
		entries:    make(map[string]*list.Element),
		recent:     list.New(),
		limit:      rate.Limit(float64(requests) / window.Seconds()),
		burst:      requests,
		evictAfter: window * 10,
		maxKeys:    maxLimiterKeys,
		now:        clock,
	}
}

// Allow counts one request for key and reports whether it is within the
// limit. For a key with an IPv6 /64 in front, the request must also be within
// its /48's limit; a request either limit refuses is taken from neither
// bucket, so a refusal never costs anything. The /48 is checked first, before
// the /64's bucket is made or even looked at: a request the /48 refuses adds
// no key and moves none, so a flood of fresh /64s from one /48 makes no more
// keys than the /48 lets through, and cannot push out another key's used-up
// bucket (which would hand that key a full one).
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.dropIdle(now)

	var wide *rate.Limiter
	if w := wideKey(key); w != "" {
		wide = l.bucket(w, l.limit*wideFactor, l.burst*wideFactor, now)
		if wide.TokensAt(now) < 1 {
			return false
		}
	}
	narrow := l.bucket(key, l.limit, l.burst, now)
	if narrow.TokensAt(now) < 1 {
		return false
	}
	narrow.AllowN(now, 1)
	if wide != nil {
		wide.AllowN(now, 1)
	}
	return true
}

// bucket returns key's bucket, made with limit and burst when key is new,
// and marks key as just seen. A new key in a full map first drops the key
// seen longest ago.
func (l *Limiter) bucket(key string, limit rate.Limit, burst int, now time.Time) *rate.Limiter {
	if el, ok := l.entries[key]; ok {
		e := el.Value.(*limiterEntry)
		e.lastSeen = now
		l.recent.MoveToFront(el)
		return e.limiter
	}
	if len(l.entries) >= l.maxKeys {
		l.remove(l.recent.Back())
	}
	e := &limiterEntry{key: key, limiter: rate.NewLimiter(limit, burst), lastSeen: now}
	l.entries[key] = l.recent.PushFront(e)
	return e.limiter
}

// dropIdle drops every key idle for longer than ten windows. The keys are in
// the order they were last seen, so it stops at the first one still in use.
func (l *Limiter) dropIdle(now time.Time) {
	for el := l.recent.Back(); el != nil; el = l.recent.Back() {
		if now.Sub(el.Value.(*limiterEntry).lastSeen) <= l.evictAfter {
			return
		}
		l.remove(el)
	}
}

func (l *Limiter) remove(el *list.Element) {
	if el == nil {
		return
	}
	delete(l.entries, el.Value.(*limiterEntry).key)
	l.recent.Remove(el)
}

// wideKey is the key that counts key's request under its /48: key with its
// leading IPv6 /64, as KeyByIP writes it, replaced by the /48 that holds it,
// and the rest of the key (a name, an account) kept. It is "" for any other
// key: an IPv4 address, a client keyed by its host (hostKey), a person.
func wideKey(key string) string {
	head, _, _ := strings.Cut(key, "|")
	network, err := netip.ParsePrefix(head)
	if err != nil || network.Bits() != 64 || !network.Addr().Is6() {
		return ""
	}
	wide, err := network.Addr().Prefix(48)
	if err != nil {
		return ""
	}
	return wide.String() + key[len(head):]
}

// RateLimit returns Gin middleware enforcing cfg with a Limiter keyed by
// cfg.KeyFunc. On rejection it answers RespondRateLimited.
func RateLimit(cfg LimitConfig) gin.HandlerFunc {
	limiter := NewLimiter(cfg.Requests, cfg.Window, cfg.Clock)
	return func(c *gin.Context) {
		// The key is read before the lock: it may decode the body.
		if !limiter.Allow(cfg.KeyFunc(c)) {
			RespondRateLimited(c)
			return
		}
		c.Next()
	}
}

// RespondRateLimited aborts with the answer every limit gives: 429 and
// {"error":"rate_limited"}.
func RespondRateLimited(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate_limited"})
}

// KeyByIP keys the limiter by the request's address: the client IP (the
// socket address; NewServer trusts no forwarded header), with an IPv6 address
// cut to its /64 network. One IPv6 client is normally given a whole /64, so
// keying each address on its own would let it walk through billions of them
// to escape every per-address and per-name limit (and the Limiter counts the
// /48 as well). An IPv4 address, also one written in IPv6 form
// (::ffff:a.b.c.d), is its own key. A client whose address gin cannot read is
// keyed by its host as the connection gives it (hostKey).
func KeyByIP(c *gin.Context) string {
	if ip := c.ClientIP(); ip != "" {
		return addressKey(ip)
	}
	return hostKey(c.Request.RemoteAddr)
}

// addressKey is KeyByIP's rule for one IP. Text that is not an IP is kept as
// it is.
func addressKey(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	addr = addr.Unmap().WithZone("")
	if addr.Is4() {
		return addr.String()
	}
	network, err := addr.Prefix(64)
	if err != nil {
		return ip
	}
	return network.String()
}

// hostKey keys a client whose address gin could not read, so such clients
// never all share one empty key: the host part of the connection's address
// (all of it when it has no port). A link-local address with a zone (gin
// reads none) is kept whole, zone and all: every link-local address is in
// fe80::/64, so cutting it to its network would put every phone on the link
// in one bucket. Any other address follows addressKey, and text that is not
// an address is kept as it is.
func hostKey(remote string) string {
	remote = strings.TrimSpace(remote)
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	if addr.Zone() != "" {
		return addr.String()
	}
	return addressKey(host)
}

// maxNameKey is the longest name a key holds as written. A longer one is
// keyed by its SHA-256, so a long name in a body can never make a large key.
const maxNameKey = 256

// nameKey is the name part of a key: trimmed and lowercased as the handlers
// compare it, and hashed when longer than maxNameKey bytes.
func nameKey(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if len(name) <= maxNameKey {
		return name
	}
	sum := sha256.Sum256([]byte(name))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// KeyByIPAndBody keys the limiter by the address (KeyByIP) combined with the
// name a JSON body is for (an email or a login), trimmed and lowercased as the
// handlers compare it, so throttling applies per account as well as per
// source and padding or capitals never make a fresh bucket. pick reads the
// name from the body decoded into T, the handler's own request type, exactly
// as the handler decodes it: encoding/json matches keys in any case and lets
// the last of two win, so reading the body any other way could count one name
// while the handler uses another. The binding rules are left to the handler,
// so a body it will refuse still counts against the name it carries. The body
// is read with ShouldBindBodyWith, which caches it so the handler can still
// bind it afterward. A body that does not decode is keyed by the address
// alone. A name over 256 bytes is keyed by its hash (nameKey).
func KeyByIPAndBody[T any](pick func(*T) string) func(*gin.Context) string {
	return func(c *gin.Context) string {
		var body T
		if err := c.ShouldBindBodyWith(&body, decodeOnly{}); err != nil {
			return KeyByIP(c)
		}
		return KeyByIP(c) + "|" + nameKey(pick(&body))
	}
}

// decodeOnly decodes a JSON body as binding.JSON does (gin's JSON codec and
// its two decoder switches) without the binding rules, which only the
// handler applies.
type decodeOnly struct{}

func (decodeOnly) Name() string { return "json" }

func (d decodeOnly) Bind(req *http.Request, obj any) error {
	if req == nil || req.Body == nil {
		return errors.New("invalid request")
	}
	return d.decode(req.Body, obj)
}

func (d decodeOnly) BindBody(body []byte, obj any) error {
	return d.decode(bytes.NewReader(body), obj)
}

func (decodeOnly) decode(r io.Reader, obj any) error {
	decoder := json.API.NewDecoder(r)
	if binding.EnableDecoderUseNumber {
		decoder.UseNumber()
	}
	if binding.EnableDecoderDisallowUnknownFields {
		decoder.DisallowUnknownFields()
	}
	return decoder.Decode(obj)
}

// KeyBySubject keys the limiter by the authenticated subject, for routes
// mounted behind RequireAuth, and by the address (KeyByIP) without one.
func KeyBySubject(c *gin.Context) string {
	subject, err := SubjectFromContext(c)
	if err != nil {
		return KeyByIP(c)
	}
	return subject.String()
}
