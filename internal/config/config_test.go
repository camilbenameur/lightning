package config

import (
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"
)

func load(env map[string]string) (Config, error) {
	return Load(func(k string) (string, bool) {
		v, ok := env[k]
		return v, ok
	})
}

func TestDefaults(t *testing.T) {
	c, err := load(nil)
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		Addr:            ":8080",
		MetricsAddr:     ":9090",
		LogLevel:        slog.LevelInfo,
		LogFormat:       "json",
		ImagePrefix:     "lightning-runtime-",
		ImageTag:        "latest",
		RunTimeout:      10 * time.Second,
		MemoryMB:        256,
		CPUs:            1,
		PidsLimit:       128,
		MaxCodeBytes:    65536,
		MaxStdinBytes:   65536,
		MaxOutputBytes:  65536,
		MaxConcurrent:   4,
		QueueTimeout:    20 * time.Second,
		RateLimit:       30,
		RateBurst:       10,
		ShutdownTimeout: 25 * time.Second,
	}
	if !reflect.DeepEqual(c, want) {
		t.Fatalf("defaults:\n got %+v\nwant %+v", c, want)
	}
}

func TestOverrides(t *testing.T) {
	c, err := load(map[string]string{
		"LIGHTNING_ADDR":                 "127.0.0.1:3000",
		"LIGHTNING_METRICS_ADDR":         "",
		"LIGHTNING_LOG_LEVEL":            "DEBUG",
		"LIGHTNING_LOG_FORMAT":           "text",
		"LIGHTNING_LANGUAGES":            " Python, go ,,bash",
		"LIGHTNING_RUNTIME_IMAGE_PREFIX": "ghcr.io/camilbenameur/lightning-runtime-",
		"LIGHTNING_RUNTIME_TAG":          "v1.2.3",
		"LIGHTNING_PULL_IMAGES":          "true",
		"LIGHTNING_SANDBOX_RUNTIME":      "runsc",
		"LIGHTNING_RUN_TIMEOUT":          "5s",
		"LIGHTNING_MEMORY_MB":            "512",
		"LIGHTNING_CPUS":                 "0.5",
		"LIGHTNING_PIDS_LIMIT":           "64",
		"LIGHTNING_MAX_CODE_BYTES":       "1000",
		"LIGHTNING_MAX_STDIN_BYTES":      "0",
		"LIGHTNING_MAX_OUTPUT_BYTES":     "2048",
		"LIGHTNING_MAX_CONCURRENT":       "8",
		"LIGHTNING_QUEUE_TIMEOUT":        "0s",
		"LIGHTNING_RATE_LIMIT":           "0",
		"LIGHTNING_RATE_BURST":           "3",
		"LIGHTNING_TRUST_PROXY":          "1",
		"LIGHTNING_CORS_ORIGINS":         "https://a.example, http://localhost:5173",
		"LIGHTNING_SHUTDOWN_TIMEOUT":     "1m",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		Addr:            "127.0.0.1:3000",
		LogLevel:        slog.LevelDebug,
		LogFormat:       "text",
		Languages:       []string{"python", "go", "bash"},
		ImagePrefix:     "ghcr.io/camilbenameur/lightning-runtime-",
		ImageTag:        "v1.2.3",
		PullImages:      true,
		SandboxRuntime:  "runsc",
		RunTimeout:      5 * time.Second,
		MemoryMB:        512,
		CPUs:            0.5,
		PidsLimit:       64,
		MaxCodeBytes:    1000,
		MaxStdinBytes:   0,
		MaxOutputBytes:  2048,
		MaxConcurrent:   8,
		QueueTimeout:    0,
		RateLimit:       0,
		RateBurst:       3,
		TrustProxy:      true,
		CORSOrigins:     []string{"https://a.example", "http://localhost:5173"},
		ShutdownTimeout: time.Minute,
	}
	if !reflect.DeepEqual(c, want) {
		t.Fatalf("overrides:\n got %+v\nwant %+v", c, want)
	}
}

func TestBlankValuesUseDefaults(t *testing.T) {
	c, err := load(map[string]string{"LIGHTNING_ADDR": " ", "LIGHTNING_RUN_TIMEOUT": "", "LIGHTNING_CORS_ORIGINS": " "})
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":8080" || c.RunTimeout != 10*time.Second || c.CORSOrigins != nil {
		t.Fatalf("unexpected config %+v", c)
	}
}

func TestWildcardOrigin(t *testing.T) {
	c, err := load(map[string]string{"LIGHTNING_CORS_ORIGINS": "*"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.CORSOrigins, []string{"*"}) {
		t.Fatalf("origins = %v", c.CORSOrigins)
	}
}

func TestInvalidValues(t *testing.T) {
	tests := map[string]string{
		"LIGHTNING_ADDR":                 "8080",
		"LIGHTNING_METRICS_ADDR":         "localhost",
		"LIGHTNING_LOG_LEVEL":            "verbose",
		"LIGHTNING_LOG_FORMAT":           "xml",
		"LIGHTNING_LANGUAGES":            "python,cobol",
		"LIGHTNING_RUNTIME_IMAGE_PREFIX": "repo:tag-",
		"LIGHTNING_RUNTIME_TAG":          "a/b",
		"LIGHTNING_PULL_IMAGES":          "maybe",
		"LIGHTNING_RUN_TIMEOUT":          "10",
		"LIGHTNING_MEMORY_MB":            "8",
		"LIGHTNING_CPUS":                 "abc",
		"LIGHTNING_PIDS_LIMIT":           "-1",
		"LIGHTNING_MAX_CODE_BYTES":       "200000",
		"LIGHTNING_MAX_STDIN_BYTES":      "1.5",
		"LIGHTNING_MAX_OUTPUT_BYTES":     "10",
		"LIGHTNING_MAX_CONCURRENT":       "0",
		"LIGHTNING_QUEUE_TIMEOUT":        "-1s",
		"LIGHTNING_RATE_LIMIT":           "-5",
		"LIGHTNING_RATE_BURST":           "0",
		"LIGHTNING_TRUST_PROXY":          "yes please",
		"LIGHTNING_CORS_ORIGINS":         "example.com",
		"LIGHTNING_SHUTDOWN_TIMEOUT":     "forever",
	}
	for key, value := range tests {
		t.Run(key, func(t *testing.T) {
			_, err := load(map[string]string{key: value})
			if err == nil || !strings.Contains(err.Error(), key) {
				t.Fatalf("%s=%q: error %v does not mention the variable", key, value, err)
			}
		})
	}
}

func TestInvalidOrigins(t *testing.T) {
	for _, origins := range []string{"*,https://a.example", "https://a.example/path", "ftp://a.example", "https://user@a.example"} {
		if _, err := load(map[string]string{"LIGHTNING_CORS_ORIGINS": origins}); err == nil {
			t.Errorf("origins %q: expected an error", origins)
		}
	}
}

func TestSameAddrs(t *testing.T) {
	_, err := load(map[string]string{"LIGHTNING_ADDR": ":9000", "LIGHTNING_METRICS_ADDR": ":9000"})
	if err == nil {
		t.Fatal("expected an error when both listeners share an address")
	}
}

func TestReportsAllErrors(t *testing.T) {
	_, err := load(map[string]string{"LIGHTNING_MEMORY_MB": "x", "LIGHTNING_CPUS": "y"})
	if err == nil || !strings.Contains(err.Error(), "LIGHTNING_MEMORY_MB") || !strings.Contains(err.Error(), "LIGHTNING_CPUS") {
		t.Fatalf("expected both errors, got %v", err)
	}
}
