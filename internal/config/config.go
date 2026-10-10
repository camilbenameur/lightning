// Package config loads and validates the service configuration from
// LIGHTNING_* environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/camilbenameur/lightning/internal/language"
)

// maxInputBytes bounds code and stdin sizes. Both travel base64-encoded in a
// single environment variable, and Linux caps one "KEY=value" string at
// 128 KiB (MAX_ARG_STRLEN), so 90 KiB of input (120 KiB encoded) is the most
// that is guaranteed to fit.
const maxInputBytes = 90 << 10

// Config is the validated service configuration.
type Config struct {
	Addr        string // API listen address
	MetricsAddr string // Prometheus listen address; empty disables it
	LogLevel    slog.Level
	LogFormat   string // "json" or "text"

	Languages      []string // enabled language IDs; empty means all
	ImagePrefix    string
	ImageTag       string
	PullImages     bool
	SandboxRuntime string // optional OCI runtime, e.g. "runsc"

	RunTimeout     time.Duration
	MemoryMB       int
	CPUs           float64
	PidsLimit      int
	MaxCodeBytes   int
	MaxStdinBytes  int
	MaxOutputBytes int

	MaxConcurrent   int
	QueueTimeout    time.Duration
	RateLimit       int // runs per minute per client; 0 disables rate limiting
	RateBurst       int
	TrustProxy      bool
	CORSOrigins     []string // allowed origins; "*" allows any
	ShutdownTimeout time.Duration
}

// Load reads the configuration through lookup (typically os.LookupEnv),
// applying defaults for unset variables. All problems are reported at once.
func Load(lookup func(string) (string, bool)) (Config, error) {
	p := &parser{lookup: lookup}
	c := Config{
		Addr:        p.addr("LIGHTNING_ADDR", ":8080", false),
		MetricsAddr: p.addr("LIGHTNING_METRICS_ADDR", ":9090", true),
		LogLevel:    p.level("LIGHTNING_LOG_LEVEL", slog.LevelInfo),
		LogFormat:   p.oneOf("LIGHTNING_LOG_FORMAT", "json", "json", "text"),

		Languages:      p.languages("LIGHTNING_LANGUAGES"),
		ImagePrefix:    p.str("LIGHTNING_RUNTIME_IMAGE_PREFIX", "lightning-runtime-"),
		ImageTag:       p.str("LIGHTNING_RUNTIME_TAG", "latest"),
		PullImages:     p.boolean("LIGHTNING_PULL_IMAGES", false),
		SandboxRuntime: p.str("LIGHTNING_SANDBOX_RUNTIME", ""),

		RunTimeout:     p.duration("LIGHTNING_RUN_TIMEOUT", 10*time.Second, time.Second, 5*time.Minute),
		MemoryMB:       p.integer("LIGHTNING_MEMORY_MB", 256, 64, 16<<10),
		CPUs:           p.float("LIGHTNING_CPUS", 1, 0.1, 64),
		PidsLimit:      p.integer("LIGHTNING_PIDS_LIMIT", 128, 16, 4096),
		MaxCodeBytes:   p.integer("LIGHTNING_MAX_CODE_BYTES", 64<<10, 1, maxInputBytes),
		MaxStdinBytes:  p.integer("LIGHTNING_MAX_STDIN_BYTES", 64<<10, 0, maxInputBytes),
		MaxOutputBytes: p.integer("LIGHTNING_MAX_OUTPUT_BYTES", 64<<10, 1<<10, 512<<10),

		MaxConcurrent:   p.integer("LIGHTNING_MAX_CONCURRENT", 4, 1, 256),
		QueueTimeout:    p.duration("LIGHTNING_QUEUE_TIMEOUT", 20*time.Second, 0, 5*time.Minute),
		RateLimit:       p.integer("LIGHTNING_RATE_LIMIT", 30, 0, 100_000),
		RateBurst:       p.integer("LIGHTNING_RATE_BURST", 10, 1, 100_000),
		TrustProxy:      p.boolean("LIGHTNING_TRUST_PROXY", false),
		CORSOrigins:     p.origins("LIGHTNING_CORS_ORIGINS"),
		ShutdownTimeout: p.duration("LIGHTNING_SHUTDOWN_TIMEOUT", 25*time.Second, 0, 10*time.Minute),
	}
	if c.ImageTag == "" || strings.ContainsAny(c.ImageTag, ":/@ ") {
		p.fail("LIGHTNING_RUNTIME_TAG", "must be a non-empty image tag")
	}
	// A registry host may carry a port ("localhost:5000/x-"), the repository name may not.
	if repo := c.ImagePrefix[strings.LastIndex(c.ImagePrefix, "/")+1:]; strings.ContainsAny(repo, ":@") ||
		strings.ContainsAny(c.ImagePrefix, " \t") {
		p.fail("LIGHTNING_RUNTIME_IMAGE_PREFIX", "must be an image name prefix without tag or digest, got %q", c.ImagePrefix)
	}
	if c.MetricsAddr != "" && c.MetricsAddr == c.Addr {
		p.fail("LIGHTNING_METRICS_ADDR", "must differ from LIGHTNING_ADDR")
	}
	return c, errors.Join(p.errs...)
}

// parser reads typed values and accumulates validation errors.
type parser struct {
	lookup func(string) (string, bool)
	errs   []error
}

func (p *parser) fail(key, format string, args ...any) {
	p.errs = append(p.errs, fmt.Errorf("%s: "+format, append([]any{key}, args...)...))
}

// get returns the trimmed value of key, or ok=false if it is unset or blank.
func (p *parser) get(key string) (string, bool) {
	v, ok := p.lookup(key)
	v = strings.TrimSpace(v)
	return v, ok && v != ""
}

func (p *parser) str(key, def string) string {
	if v, ok := p.get(key); ok {
		return v
	}
	return def
}

func (p *parser) oneOf(key, def string, allowed ...string) string {
	v := strings.ToLower(p.str(key, def))
	for _, a := range allowed {
		if v == a {
			return v
		}
	}
	p.fail(key, "must be one of %s, got %q", strings.Join(allowed, ", "), v)
	return def
}

func (p *parser) boolean(key string, def bool) bool {
	v, ok := p.get(key)
	if !ok {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		p.fail(key, "must be a boolean, got %q", v)
		return def
	}
	return b
}

func (p *parser) integer(key string, def, lo, hi int) int {
	v, ok := p.get(key)
	if !ok {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < lo || n > hi {
		p.fail(key, "must be an integer between %d and %d, got %q", lo, hi, v)
		return def
	}
	return n
}

func (p *parser) float(key string, def, lo, hi float64) float64 {
	v, ok := p.get(key)
	if !ok {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f < lo || f > hi {
		p.fail(key, "must be a number between %g and %g, got %q", lo, hi, v)
		return def
	}
	return f
}

func (p *parser) duration(key string, def, lo, hi time.Duration) time.Duration {
	v, ok := p.get(key)
	if !ok {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < lo || d > hi {
		p.fail(key, "must be a duration between %v and %v (e.g. \"10s\"), got %q", lo, hi, v)
		return def
	}
	return d
}

func (p *parser) level(key string, def slog.Level) slog.Level {
	v, ok := p.get(key)
	if !ok {
		return def
	}
	var l slog.Level
	if err := l.UnmarshalText([]byte(v)); err != nil {
		p.fail(key, "must be one of debug, info, warn, error, got %q", v)
		return def
	}
	return l
}

func (p *parser) addr(key, def string, optional bool) string {
	v, set := p.lookup(key)
	if v = strings.TrimSpace(v); v == "" {
		if set && optional {
			return ""
		}
		v = def
	}
	if _, port, err := net.SplitHostPort(v); err != nil || port == "" {
		p.fail(key, "must be a host:port listen address such as \":8080\", got %q", v)
		return def
	}
	return v
}

func list(v string) []string {
	var out []string
	for item := range strings.SplitSeq(v, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func (p *parser) languages(key string) []string {
	v, _ := p.get(key)
	ids := list(strings.ToLower(v))
	if _, err := language.NewRegistry(ids); err != nil {
		p.fail(key, "%v", err)
		return nil
	}
	return ids
}

func (p *parser) origins(key string) []string {
	v, _ := p.get(key)
	origins := list(v)
	for _, o := range origins {
		if o == "*" {
			if len(origins) > 1 {
				p.fail(key, "\"*\" cannot be combined with other origins")
				return nil
			}
			continue
		}
		u, err := url.Parse(o)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
			u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			p.fail(key, "%q is not an origin like https://example.com", o)
			return nil
		}
	}
	return origins
}
