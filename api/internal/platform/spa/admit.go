package spa

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"tools.xdoubleu.com/internal/platform/middleware"
)

const (
	maxTrackedClients = 10_000
	limiterBurstRatio = 2
)

// limiter is a per-client-IP token bucket for the public beacons, which sit
// outside the api's own rate limiter.
type limiter struct {
	mu      sync.Mutex
	clients map[string]*rate.Limiter
	rate    rate.Limit
	burst   int
}

func newLimiter(perMinute int) *limiter {
	return &limiter{
		mu:      sync.Mutex{},
		clients: map[string]*rate.Limiter{},
		rate:    rate.Every(time.Minute / time.Duration(perMinute)),
		burst:   perMinute / limiterBurstRatio,
	}
}

func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	lim, ok := l.clients[key]
	if !ok {
		if len(l.clients) >= maxTrackedClients {
			clear(l.clients)
		}
		lim = rate.NewLimiter(l.rate, l.burst)
		l.clients[key] = lim
	}
	return lim.Allow()
}

// admit applies the checks every public beacon shares: same-origin only, a
// bounded body, and a per-IP rate. It writes the rejection itself.
func (h *Handler) admit(
	w http.ResponseWriter, r *http.Request, l *limiter, maxBody int64,
) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" {
		w.WriteHeader(http.StatusForbidden)
		return false
	}
	ip, err := middleware.ClientIP(r)
	if err != nil {
		ip = r.RemoteAddr
	}
	if !l.allow(ip) {
		w.WriteHeader(http.StatusTooManyRequests)
		return false
	}
	if r.ContentLength > maxBody {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	return true
}
