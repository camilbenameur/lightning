package server

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/netip"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"
)

type requestIDKey struct{}

// requestID propagates a well-formed incoming X-Request-ID or generates one,
// and echoes it on the response.
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !validRequestID(id) {
			id = rand.Text()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

func validRequestID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, c := range id {
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9', strings.ContainsRune("-_.:", c):
		default:
			return false
		}
	}
	return true
}

func requestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// responseRecorder captures the status and size of a response.
type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (r *responseRecorder) WriteHeader(status int) {
	if r.status != 0 {
		return // the response is committed; ignore late attempts to change it
	}
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(p)
	r.bytes += int64(n)
	return n, err
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (r *responseRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// statusClientClosed is logged when the client went away before a response was written.
const statusClientClosed = 499

// observe logs every request and records HTTP metrics.
func (s *server) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w}
		// Deferred to also record the responses aborted with http.ErrAbortHandler.
		defer func() { s.record(r, rec, time.Since(start)) }()
		next.ServeHTTP(rec, r)
	})
}

// record logs a served request and counts it in the metrics.
func (s *server) record(r *http.Request, rec *responseRecorder, elapsed time.Duration) {
	status := rec.status
	if status == 0 {
		status = http.StatusOK
		if r.Context().Err() != nil {
			status = statusClientClosed
		}
	}
	// r.Pattern is set by the ServeMux on this same request value.
	route := r.Pattern
	if route == "" || route == "/" {
		route = "unmatched"
	}
	s.Metrics.HTTPRequest(route, status, elapsed)

	level := slog.LevelInfo
	switch {
	case status == http.StatusServiceUnavailable: // busy or engine down: already explained by the handler
		level = slog.LevelWarn
	case status >= http.StatusInternalServerError:
		level = slog.LevelError
	case r.URL.Path == "/healthz" || r.URL.Path == "/readyz":
		level = slog.LevelDebug
	}
	s.Logger.LogAttrs(r.Context(), level, "request",
		slog.String("requestId", requestIDFrom(r.Context())),
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("route", route),
		slog.Int("status", status),
		slog.Int64("bytes", rec.bytes),
		slog.Float64("durationMs", float64(elapsed.Microseconds())/1000),
		slog.String("clientIp", clientIP(r, s.TrustProxy).String()),
		slog.String("userAgent", r.UserAgent()),
	)
}

// recoverPanic turns a panicking handler into a 500 response.
func (s *server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(v) // let net/http abort the response silently
			}
			s.logger(r).Error("panic serving request", "panic", v, "stack", string(debug.Stack()))
			writeError(w, errInternal)
		}()
		next.ServeHTTP(w, r)
	})
}

func methodNotAllowed(allowed string) http.HandlerFunc {
	if allowed == http.MethodGet {
		allowed += ", " + http.MethodHead
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", allowed)
		writeError(w, errorf(http.StatusMethodNotAllowed, "method_not_allowed", "method %s is not allowed on this route", r.Method))
	}
}

// cors allows cross-origin requests from the given origins ("*" for any).
func cors(origins []string, next http.Handler) http.Handler {
	anyOrigin := slices.Contains(origins, "*")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		if !anyOrigin {
			h.Add("Vary", "Origin")
		}
		origin := r.Header.Get("Origin")
		allowed := anyOrigin || slices.Contains(origins, origin)
		if origin == "" || !allowed {
			next.ServeHTTP(w, r)
			return
		}
		if anyOrigin {
			h.Set("Access-Control-Allow-Origin", "*")
		} else {
			h.Set("Access-Control-Allow-Origin", origin)
		}
		h.Set("Access-Control-Expose-Headers", "Retry-After, X-Request-ID")
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			h.Set("Access-Control-Allow-Methods", "GET, POST")
			h.Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID")
			h.Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// limitRate applies the per-client rate limit.
func (s *server) limitRate(next http.HandlerFunc) http.HandlerFunc {
	if s.limiter == nil {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ok, wait := s.limiter.Allow(rateKey(clientIP(r, s.TrustProxy)))
		if !ok {
			seconds := max(1, int(math.Ceil(wait.Seconds())))
			w.Header().Set("Retry-After", strconv.Itoa(seconds))
			writeError(w, errorf(http.StatusTooManyRequests, "rate_limited", "too many runs, retry in %d s", seconds))
			return
		}
		next(w, r)
	}
}

// clientIP returns the address of the client. Behind a trusted reverse proxy
// that is the right-most X-Forwarded-For entry, the one added by the proxy
// itself; entries further left are client-controlled.
func clientIP(r *http.Request, trustProxy bool) netip.Addr {
	if trustProxy {
		if values := r.Header.Values("X-Forwarded-For"); len(values) > 0 {
			last := values[len(values)-1]
			if addr, ok := parseAddr(last[strings.LastIndexByte(last, ',')+1:]); ok {
				return addr
			}
		}
	}
	addr, _ := parseAddr(r.RemoteAddr)
	return addr
}

// parseAddr parses an IP address with or without a port.
func parseAddr(s string) (netip.Addr, bool) {
	s = strings.TrimSpace(s)
	if ap, err := netip.ParseAddrPort(s); err == nil {
		return ap.Addr().Unmap(), true
	}
	addr, err := netip.ParseAddr(s)
	return addr.Unmap(), err == nil
}

// rateKey groups IPv6 clients by /64, the block typically assigned to a
// single subscriber, so rotating addresses within it does not evade limits.
func rateKey(addr netip.Addr) string {
	switch {
	case !addr.IsValid():
		return "unknown"
	case addr.Is6():
		prefix, _ := addr.WithZone("").Prefix(64)
		return prefix.String()
	default:
		return addr.String()
	}
}
