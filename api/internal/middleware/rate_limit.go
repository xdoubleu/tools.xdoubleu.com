package middleware

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/time/rate"

	"tools.xdoubleu.com/internal/communication/httptools"
	"tools.xdoubleu.com/internal/errortools"
)

var cleanerActive bool                   //nolint: gochecknoglobals //need this
var mu sync.RWMutex                      //nolint: gochecknoglobals //need this
var clients = make(map[string]*client)   //nolint: gochecknoglobals //need this
var errWriter = connect.NewErrorWriter() //nolint: gochecknoglobals //need this

type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimit rate limits requests per client IP.
func RateLimit(
	rps rate.Limit,
	bucketSize int,
	cleanupTimer time.Duration,
	removeAfter time.Duration,
) func(http.Handler) http.Handler {
	if !cleanerActive {
		cleanerActive = true
		go func() {
			for {
				time.Sleep(cleanupTimer)

				mu.RLock()

				for ip, client := range clients {
					if time.Since(client.lastSeen) > removeAfter {
						mu.RUnlock()
						mu.Lock()

						delete(clients, ip)

						mu.Unlock()
						mu.RLock()
					}
				}

				mu.RUnlock()
			}
		}()
	}

	return func(next http.Handler) http.Handler {
		return rateLimit(&mu, clients, rps, bucketSize, next)
	}
}

func rateLimit(
	mu *sync.RWMutex,
	clients map[string]*client,
	rps rate.Limit,
	bucketSize int,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, err := ClientIP(r)
		if err != nil {
			httptools.ServerErrorResponse(w, r, err)
			return
		}

		mu.Lock()

		if _, found := clients[ip]; !found {
			//nolint:exhaustruct //lastSeen is set later
			clients[ip] = &client{
				limiter: rate.NewLimiter(rps, bucketSize),
			}
		}

		clients[ip].lastSeen = time.Now()

		if !clients[ip].limiter.Allow() {
			mu.Unlock()

			// GET is only a Connect request with idempotency_level, which no proto sets,
			// so plain GET endpoints stay on the REST error path.
			if r.Method == http.MethodPost && errWriter.IsSupported(r) {
				connectErr := connect.NewError(
					connect.CodeResourceExhausted,
					errors.New(errortools.MessageTooManyRequests),
				)
				_ = errWriter.Write(w, r, connectErr)
				return
			}

			httptools.RateLimitExceededResponse(w, r)
			return
		}

		mu.Unlock()

		next.ServeHTTP(w, r)
	})
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
