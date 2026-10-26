package middleware

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"golang.org/x/time/rate"

	"tools.xdoubleu.com/internal/communication/httptools"
	"tools.xdoubleu.com/internal/errortools"
)

var errWriter = connecthttp.NewErrorWriter() //nolint: gochecknoglobals //need this

type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// ipLimiter holds one token bucket per client IP. Buckets idle for longer than
// removeAfter are dropped by the request path, at most every sweepEvery, so
// there is no background goroutine to stop.
type ipLimiter struct {
	rps         rate.Limit
	bucketSize  int
	sweepEvery  time.Duration
	removeAfter time.Duration
	now         func() time.Time

	mu        sync.Mutex
	clients   map[string]*client
	lastSweep time.Time
}

func newIPLimiter(
	rps rate.Limit,
	bucketSize int,
	sweepEvery time.Duration,
	removeAfter time.Duration,
	now func() time.Time,
) *ipLimiter {
	return &ipLimiter{
		rps:         rps,
		bucketSize:  bucketSize,
		sweepEvery:  sweepEvery,
		removeAfter: removeAfter,
		now:         now,
		mu:          sync.Mutex{},
		clients:     make(map[string]*client),
		lastSweep:   now(),
	}
}

// RateLimit rate limits requests per client IP. Every call has its own table of
// clients, shared by all handlers the returned function wraps. Clients unseen
// for removeAfter are forgotten, checked at most every cleanupTimer.
func RateLimit(
	rps rate.Limit,
	bucketSize int,
	cleanupTimer time.Duration,
	removeAfter time.Duration,
) func(http.Handler) http.Handler {
	return newIPLimiter(rps, bucketSize, cleanupTimer, removeAfter, time.Now).handler
}

func (l *ipLimiter) handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, err := ClientIP(r)
		if err != nil {
			httptools.ServerErrorResponse(w, r, err)
			return
		}

		if !l.allow(ip) {
			// GET is only a Connect request with idempotency_level, which no proto sets,
			// so plain GET endpoints stay on the REST error path.
			if r.Method == http.MethodPost && errWriter.IsSupported(r) {
				connectErr := connect.NewError(
					connect.CodeResourceExhausted,
					errortools.MessageTooManyRequests,
				)
				_ = errWriter.Write(w, r, connectErr)
				return
			}

			httptools.RateLimitExceededResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// allow spends one token of ip's bucket and reports whether one was left.
func (l *ipLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if now.Sub(l.lastSweep) >= l.sweepEvery {
		l.sweep(now)
	}

	c, found := l.clients[ip]
	if !found {
		//nolint:exhaustruct //lastSeen is set below
		c = &client{limiter: rate.NewLimiter(l.rps, l.bucketSize)}
		l.clients[ip] = c
	}

	c.lastSeen = now

	return c.limiter.AllowN(now, 1)
}

func (l *ipLimiter) sweep(now time.Time) {
	for ip, c := range l.clients {
		if now.Sub(c.lastSeen) > l.removeAfter {
			delete(l.clients, ip)
		}
	}

	l.lastSweep = now
}

// ClientIP is the request's client address. Behind kamal-proxy (a private or
// loopback peer) it is the right-most X-Forwarded-For hop, which the proxy
// appended; earlier hops are client-supplied. A public peer's header is
// ignored.
func ClientIP(r *http.Request) (string, error) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", err
	}
	peer, err := netip.ParseAddr(host)
	if err != nil || (!peer.IsPrivate() && !peer.IsLoopback()) {
		return host, nil //nolint:nilerr // an unparseable peer is its own key
	}

	hops := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	last := strings.TrimSpace(hops[len(hops)-1])
	if forwarded, parseErr := netip.ParseAddr(last); parseErr == nil {
		return forwarded.String(), nil
	}
	return host, nil
}
