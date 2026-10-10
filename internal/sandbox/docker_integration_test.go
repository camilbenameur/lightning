//go:build integration

// Integration tests against a real Docker Engine and the runtime images.
// Tests of a language are skipped when its image is not present locally.
// LIGHTNING_RUNTIME_IMAGE_PREFIX and LIGHTNING_RUNTIME_TAG select the images.
package sandbox

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/client"

	"github.com/camilbenameur/lightning/internal/language"
)

// recordingAPI remembers the containers it created.
type recordingAPI struct {
	dockerAPI
	mu  sync.Mutex
	ids []string
}

func (r *recordingAPI) ContainerCreate(ctx context.Context, opts client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
	res, err := r.dockerAPI.ContainerCreate(ctx, opts)
	if err == nil {
		r.mu.Lock()
		r.ids = append(r.ids, res.ID)
		r.mu.Unlock()
	}
	return res, err
}

type harness struct {
	t      *testing.T
	cli    *client.Client
	api    *recordingAPI
	docker *Docker
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	cli, err := client.New(client.FromEnv)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cli.Close() })
	if _, err := cli.Ping(t.Context(), client.PingOptions{}); err != nil {
		t.Fatalf("docker engine unreachable: %v", err)
	}
	api := &recordingAPI{dockerAPI: cli}
	h := &harness{t: t, cli: cli, api: api, docker: &Docker{api: api, runtime: os.Getenv("LIGHTNING_SANDBOX_RUNTIME"), log: slog.New(slog.NewTextHandler(os.Stderr, nil))}}
	t.Cleanup(h.assertRemoved)
	return h
}

// assertRemoved checks that every container created by the test is gone.
func (h *harness) assertRemoved() {
	h.api.mu.Lock()
	defer h.api.mu.Unlock()
	for _, id := range h.api.ids {
		_, err := h.api.ContainerInspect(context.Background(), id, client.ContainerInspectOptions{})
		if !cerrdefs.IsNotFound(err) {
			h.t.Errorf("container %s was not removed (inspect error: %v)", id, err)
		}
	}
}

func imageRef(id string) string {
	prefix, tag := os.Getenv("LIGHTNING_RUNTIME_IMAGE_PREFIX"), os.Getenv("LIGHTNING_RUNTIME_TAG")
	if prefix == "" {
		prefix = "lightning-runtime-"
	}
	if tag == "" {
		tag = "latest"
	}
	return prefix + id + ":" + tag
}

// spec returns a spec with the production defaults for lang, skipping the
// test when the runtime image is missing.
func (h *harness) spec(lang, code string) Spec {
	h.t.Helper()
	image := imageRef(lang)
	missing, err := h.docker.MissingImages(h.t.Context(), []string{image})
	if err != nil {
		h.t.Fatal(err)
	}
	if len(missing) > 0 {
		h.t.Skipf("runtime image %s is not available", image)
	}
	var l language.Language
	for _, candidate := range language.All() {
		if candidate.ID == lang {
			l = candidate
		}
	}
	return Spec{
		Image:       image,
		Source:      code,
		MemoryBytes: int64(l.MemoryMB(256)) << 20,
		NanoCPUs:    1e9,
		PidsLimit:   128,
		Timeout:     l.Timeout(10 * time.Second),
		MaxOutput:   64 << 10,
	}
}

// timedSink records output with arrival times.
type timedSink struct {
	mu       sync.Mutex
	started  time.Time
	chunks   []chunk
	firstOut time.Time
}

func (s *timedSink) Started() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.started = time.Now()
}

func (s *timedSink) Output(stream Stream, p []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.firstOut.IsZero() {
		s.firstOut = time.Now()
	}
	s.chunks = append(s.chunks, chunk{stream, string(p)})
}

func (s *timedSink) text(stream Stream) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var b strings.Builder
	for _, c := range s.chunks {
		if c.stream == stream {
			b.WriteString(c.data)
		}
	}
	return b.String()
}

func (h *harness) run(spec Spec) (*timedSink, Result) {
	h.t.Helper()
	sink := &timedSink{}
	res, err := h.docker.Run(h.t.Context(), spec, sink)
	if err != nil {
		h.t.Fatalf("run failed: %v", err)
	}
	if sink.started.IsZero() {
		h.t.Fatal("Started was not called")
	}
	return sink, res
}

var programs = map[string]struct{ hello, upper string }{
	"python": {
		hello: `print("Hello, world!")`,
		upper: "import sys\nprint(sys.stdin.read().upper(), end=\"\")",
	},
	"javascript": {
		hello: `console.log("Hello, world!");`,
		upper: `process.stdout.write(require("fs").readFileSync(0, "utf8").toUpperCase());`,
	},
	"typescript": {
		hello: "enum Greeting { Hello = \"Hello, world!\" }\nconst message: string = Greeting.Hello;\nconsole.log(message);",
		upper: "import { readFileSync } from \"node:fs\";\nconst input: string = readFileSync(0, \"utf8\");\nprocess.stdout.write(input.toUpperCase());",
	},
	"go": {
		hello: "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"Hello, world!\") }\n",
		upper: "package main\n\nimport (\n\t\"io\"\n\t\"os\"\n\t\"strings\"\n)\n\nfunc main() {\n\tb, _ := io.ReadAll(os.Stdin)\n\tos.Stdout.WriteString(strings.ToUpper(string(b)))\n}\n",
	},
	"rust": {
		hello: "fn main() {\n    println!(\"Hello, world!\");\n}\n",
		upper: "use std::io::Read;\n\nfn main() {\n    let mut s = String::new();\n    std::io::stdin().read_to_string(&mut s).unwrap();\n    print!(\"{}\", s.to_uppercase());\n}\n",
	},
	"c": {
		hello: "#include <stdio.h>\n\nint main(void) {\n    puts(\"Hello, world!\");\n    return 0;\n}\n",
		upper: "#include <ctype.h>\n#include <stdio.h>\n\nint main(void) {\n    int c;\n    while ((c = getchar()) != EOF) putchar(toupper(c));\n    return 0;\n}\n",
	},
	"cpp": {
		hello: "#include <iostream>\n\nint main() {\n    std::cout << \"Hello, world!\" << std::endl;\n}\n",
		upper: "#include <cctype>\n#include <iostream>\n\nint main() {\n    char c;\n    while (std::cin.get(c)) std::cout.put(static_cast<char>(std::toupper(static_cast<unsigned char>(c))));\n}\n",
	},
	"java": {
		hello: "public class Main {\n    public static void main(String[] args) {\n        System.out.println(\"Hello, world!\");\n    }\n}\n",
		upper: "public class Main {\n    public static void main(String[] args) throws Exception {\n        System.out.print(new String(System.in.readAllBytes()).toUpperCase());\n    }\n}\n",
	},
	"ruby": {
		hello: `puts "Hello, world!"`,
		upper: `print STDIN.read.upcase`,
	},
	"php": {
		hello: "<?php\necho \"Hello, world!\\n\";\n",
		upper: "<?php\necho strtoupper(stream_get_contents(STDIN));\n",
	},
	"bash": {
		hello: `echo "Hello, world!"`,
		upper: `tr '[:lower:]' '[:upper:]'`,
	},
}

func TestIntegrationLanguages(t *testing.T) {
	// These check the runtime contract, not latency (which is logged): a
	// generous timeout keeps them from flaking on a busy host.
	const timeout = time.Minute
	for _, l := range language.All() {
		prog, ok := programs[l.ID]
		if !ok {
			t.Fatalf("no test program for %s", l.ID)
		}
		t.Run(l.ID+"/hello", func(t *testing.T) {
			h := newHarness(t)
			spec := h.spec(l.ID, prog.hello)
			spec.Timeout = timeout
			sink, res := h.run(spec)
			if res.ExitCode != 0 || sink.text(Stdout) != "Hello, world!\n" {
				t.Fatalf("result %+v, stdout %q, stderr %q", res, sink.text(Stdout), sink.text(Stderr))
			}
			t.Logf("%s hello world: %v", l.ID, res.Duration)
		})
		t.Run(l.ID+"/stdin", func(t *testing.T) {
			h := newHarness(t)
			spec := h.spec(l.ID, prog.upper)
			spec.Timeout = timeout
			spec.Stdin = "hello lightning\nsecond line\n"
			sink, res := h.run(spec)
			if res.ExitCode != 0 || sink.text(Stdout) != "HELLO LIGHTNING\nSECOND LINE\n" {
				t.Fatalf("result %+v, stdout %q, stderr %q", res, sink.text(Stdout), sink.text(Stderr))
			}
		})
	}
}

func TestIntegrationExitCodeAndStderr(t *testing.T) {
	h := newHarness(t)
	sink, res := h.run(h.spec("python", "import sys\nprint('bye')\nprint('oops', file=sys.stderr)\nsys.exit(3)"))
	if res.ExitCode != 3 || sink.text(Stdout) != "bye\n" || sink.text(Stderr) != "oops\n" {
		t.Fatalf("result %+v, stdout %q, stderr %q", res, sink.text(Stdout), sink.text(Stderr))
	}
	if res.TimedOut || res.OOMKilled || res.Truncated {
		t.Fatalf("result = %+v", res)
	}
}

func TestIntegrationCompileError(t *testing.T) {
	h := newHarness(t)
	sink, res := h.run(h.spec("c", "int main( {"))
	if res.ExitCode == 0 || sink.text(Stderr) == "" || sink.text(Stdout) != "" {
		t.Fatalf("result %+v, stdout %q, stderr %q", res, sink.text(Stdout), sink.text(Stderr))
	}
}

func TestIntegrationTimeout(t *testing.T) {
	h := newHarness(t)
	spec := h.spec("python", "print('spinning', flush=True)\nwhile True:\n    pass")
	spec.Timeout = 2 * time.Second
	start := time.Now()
	sink, res := h.run(spec)
	if !res.TimedOut || res.ExitCode != 137 || sink.text(Stdout) != "spinning\n" {
		t.Fatalf("result %+v, stdout %q", res, sink.text(Stdout))
	}
	if elapsed := time.Since(start); elapsed > spec.Timeout+10*time.Second {
		t.Fatalf("run took %v", elapsed)
	}
}

func TestIntegrationNetworkIsDisabled(t *testing.T) {
	h := newHarness(t)
	code := `import os, socket
print(sorted(os.listdir("/sys/class/net")))
try:
    socket.create_connection(("1.1.1.1", 53), timeout=3)
    print("connected")
except OSError as e:
    print("blocked")
`
	sink, res := h.run(h.spec("python", code))
	if res.ExitCode != 0 || sink.text(Stdout) != "['lo']\nblocked\n" {
		t.Fatalf("result %+v, stdout %q, stderr %q", res, sink.text(Stdout), sink.text(Stderr))
	}
}

func TestIntegrationOOM(t *testing.T) {
	h := newHarness(t)
	info, err := h.cli.Info(t.Context(), client.InfoOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !info.Info.SwapLimit {
		t.Skip("the engine cannot limit swap: memory beyond the limit is swapped out rather than OOM-killed")
	}
	sink, res := h.run(h.spec("python", "blocks = []\nwhile True:\n    blocks.append(b'x' * (16 << 20))"))
	if !res.OOMKilled || res.ExitCode != 137 || res.TimedOut {
		t.Fatalf("result %+v, stderr %q", res, sink.text(Stderr))
	}
}

func TestIntegrationOutputLimit(t *testing.T) {
	h := newHarness(t)
	spec := h.spec("python", "while True:\n    print('y' * 99)")
	sink, res := h.run(spec)
	if !res.Truncated || res.TimedOut {
		t.Fatalf("result = %+v", res)
	}
	if got := len(sink.text(Stdout)); got != spec.MaxOutput {
		t.Fatalf("output length = %d, want %d", got, spec.MaxOutput)
	}
}

// TestIntegrationOutputFlood floods the log driver with tiny lines: the output
// limit is then reached after seconds, and the engine takes seconds more to
// process the kill, often past the deadline, which must not turn the run into
// a timeout.
func TestIntegrationOutputFlood(t *testing.T) {
	h := newHarness(t)
	spec := h.spec("bash", "yes")
	start := time.Now()
	sink, res := h.run(spec)
	if !res.Truncated || res.TimedOut || res.ExitCode != 137 || len(sink.text(Stdout)) != spec.MaxOutput {
		t.Fatalf("result %+v, %d bytes of output", res, len(sink.text(Stdout)))
	}
	t.Logf("killed %v after start; the run took %v", res.Duration, time.Since(start))
}

func TestIntegrationForkBombIsContained(t *testing.T) {
	h := newHarness(t)
	code := `import os
while True:
    try:
        os.fork()
    except OSError:
        pass
`
	spec := h.spec("python", code)
	spec.Timeout = 3 * time.Second
	start := time.Now()
	_, res := h.run(spec)
	if !res.TimedOut {
		t.Fatalf("result = %+v", res)
	}
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Fatalf("killing the fork bomb took %v", elapsed)
	}
}

func TestIntegrationSandboxPosture(t *testing.T) {
	h := newHarness(t)
	code := `import os, socket
status = dict(line.split(":\t", 1) for line in open("/proc/self/status").read().splitlines() if ":\t" in line)
root = [l.split()[3].split(",")[0] for l in open("/proc/mounts") if l.split()[1] == "/"][0]
print("uid", os.getuid(), os.getgid())
print("caps", status["CapEff"].strip(), "nnp", status["NoNewPrivs"].strip())
print("root", root, "host", socket.gethostname(), "cwd", os.getcwd())
for path in ["/usr/local/escape", "/workspace/f", "/tmp/f"]:
    try:
        with open(path, "w") as f:
            f.write("x")
        print(path, "writable")
    except OSError:
        print(path, "denied")
print("env", "LIGHTNING_SOURCE_B64" in os.environ, "LIGHTNING_STDIN_B64" in os.environ)
`
	sink, res := h.run(h.spec("python", code))
	want := `uid 10001 10001
caps 0000000000000000 nnp 1
root ro host sandbox cwd /workspace
/usr/local/escape denied
/workspace/f writable
/tmp/f writable
env False False
`
	if res.ExitCode != 0 || sink.text(Stdout) != want {
		t.Fatalf("exit %d\nstdout:\n%s\nwant:\n%s\nstderr: %s", res.ExitCode, sink.text(Stdout), want, sink.text(Stderr))
	}
}

func TestIntegrationStreamingOrder(t *testing.T) {
	h := newHarness(t)
	code := `import sys, time
for i in range(3):
    print(f"out{i}", flush=True)
    time.sleep(0.1)
    print(f"err{i}", file=sys.stderr, flush=True)
    time.sleep(0.1)
`
	start := time.Now()
	sink, res := h.run(h.spec("python", code))
	end := time.Now()
	if res.ExitCode != 0 {
		t.Fatalf("exit %d, stderr %q", res.ExitCode, sink.text(Stderr))
	}
	var got []string
	for _, c := range sink.chunks {
		got = append(got, c.stream.String()+":"+c.data)
	}
	want := "stdout:out0\n stderr:err0\n stdout:out1\n stderr:err1\n stdout:out2\n stderr:err2\n"
	if strings.Join(got, " ") != want {
		t.Fatalf("events = %q", got)
	}
	if lead := end.Sub(sink.firstOut); lead < 400*time.Millisecond {
		t.Fatalf("first output arrived only %v before the end (run started %v earlier): not streamed", lead, end.Sub(start))
	}
}

func TestIntegrationUnicodeAndLargeSource(t *testing.T) {
	h := newHarness(t)
	source := "# " + strings.Repeat("x", 64<<10-64) + "\nimport sys\nsys.stdout.write(sys.stdin.read()[::-1])\n"
	if len(source) > 64<<10 {
		t.Fatalf("source is %d bytes", len(source))
	}
	spec := h.spec("python", source)
	spec.Stdin = "héllo 😀 日本"
	sink, res := h.run(spec)
	if res.ExitCode != 0 || sink.text(Stdout) != "本日 😀 olléh" {
		t.Fatalf("result %+v, stdout %q, stderr %q", res, sink.text(Stdout), sink.text(Stderr))
	}
}

func TestIntegrationCancellationKillsContainer(t *testing.T) {
	h := newHarness(t)
	spec := h.spec("python", "import time\nprint('ready', flush=True)\ntime.sleep(60)")
	ctx, cancel := context.WithCancel(t.Context())
	sink := &cancelOnOutput{cancel: cancel}

	start := time.Now()
	_, err := h.docker.Run(ctx, spec, sink)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Fatalf("cancellation took %v", elapsed)
	}
	// harness cleanup asserts that the container is gone
}

type cancelOnOutput struct{ cancel context.CancelFunc }

func (c *cancelOnOutput) Started()              {}
func (c *cancelOnOutput) Output(Stream, []byte) { c.cancel() }

func TestIntegrationMissingImage(t *testing.T) {
	h := newHarness(t)
	spec := Spec{Image: "lightning-runtime-does-not-exist:latest", Source: "x", MemoryBytes: 64 << 20, NanoCPUs: 1e9, PidsLimit: 16, Timeout: time.Second, MaxOutput: 1024}
	_, err := h.docker.Run(t.Context(), spec, &timedSink{})
	if !errors.Is(err, ErrImageNotFound) {
		t.Fatalf("err = %v, want ErrImageNotFound", err)
	}
}
