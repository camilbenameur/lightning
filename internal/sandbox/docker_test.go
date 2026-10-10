package sandbox

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// fakeDocker emulates the engine for a single container whose "program" is a
// Go function writing to a multiplexed log stream.
type fakeDocker struct {
	dockerAPI // methods not overridden below are not expected to be called

	program   func(p *fakeProcess) int
	createErr error
	startErr  error
	oom       bool
	// Like the engine after a flood of output: a kill is answered killDelay
	// late, and an exit is reported exitDelay after it happened.
	killDelay time.Duration
	exitDelay time.Duration

	proc       *fakeProcess
	logsReader *io.PipeReader
	exit       chan container.WaitResponse

	mu       sync.Mutex
	created  []client.ContainerCreateOptions
	removed  []string
	kills    int
	started  time.Time
	finished time.Time
}

type fakeProcess struct {
	logs     *io.PipeWriter
	killed   chan struct{}
	killOnce sync.Once
}

func (p *fakeProcess) write(s stdcopy.StdType, data string) error {
	frame := make([]byte, 8, 8+len(data))
	frame[0] = byte(s)
	binary.BigEndian.PutUint32(frame[4:], uint32(len(data)))
	_, err := p.logs.Write(append(frame, data...))
	return err
}

func (p *fakeProcess) kill() { p.killOnce.Do(func() { close(p.killed) }) }

// untilKilled blocks like a program that never exits on its own.
func (p *fakeProcess) untilKilled() int {
	<-p.killed
	return exitCodeKilled
}

func newFake(program func(p *fakeProcess) int) *fakeDocker {
	pr, pw := io.Pipe()
	return &fakeDocker{
		program:    program,
		proc:       &fakeProcess{logs: pw, killed: make(chan struct{})},
		logsReader: pr,
		exit:       make(chan container.WaitResponse, 1),
	}
}

func (f *fakeDocker) ContainerCreate(_ context.Context, opts client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return client.ContainerCreateResult{}, f.createErr
	}
	f.created = append(f.created, opts)
	return client.ContainerCreateResult{ID: "c1"}, nil
}

func (f *fakeDocker) ContainerWait(_ context.Context, _ string, opts client.ContainerWaitOptions) client.ContainerWaitResult {
	if opts.Condition != container.WaitConditionNextExit {
		panic("the executor must wait for the next exit")
	}
	return client.ContainerWaitResult{Result: f.exit, Error: make(chan error)}
}

func (f *fakeDocker) ContainerStart(context.Context, string, client.ContainerStartOptions) (client.ContainerStartResult, error) {
	if f.startErr != nil {
		return client.ContainerStartResult{}, f.startErr
	}
	f.mu.Lock()
	f.started = time.Now()
	f.mu.Unlock()
	go func() {
		code := f.program(f.proc)
		_ = f.proc.logs.Close()
		f.mu.Lock()
		f.finished = time.Now()
		f.mu.Unlock()
		time.Sleep(f.exitDelay)
		f.exit <- container.WaitResponse{StatusCode: int64(code)}
	}()
	return client.ContainerStartResult{}, nil
}

func (f *fakeDocker) ContainerLogs(ctx context.Context, _ string, opts client.ContainerLogsOptions) (client.ContainerLogsResult, error) {
	if !opts.Follow || !opts.ShowStdout || !opts.ShowStderr {
		panic("the executor must follow both streams")
	}
	// Like the real client, close the stream when the context ends.
	go func() {
		<-ctx.Done()
		_ = f.logsReader.CloseWithError(ctx.Err())
	}()
	return f.logsReader, nil
}

func (f *fakeDocker) ContainerKill(context.Context, string, client.ContainerKillOptions) (client.ContainerKillResult, error) {
	f.mu.Lock()
	f.kills++
	f.mu.Unlock()
	time.Sleep(f.killDelay)
	f.proc.kill()
	return client.ContainerKillResult{}, nil
}

func (f *fakeDocker) killCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.kills
}

func (f *fakeDocker) ContainerInspect(context.Context, string, client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return client.ContainerInspectResult{Container: container.InspectResponse{State: &container.State{
		OOMKilled:  f.oom,
		StartedAt:  f.started.Format(time.RFC3339Nano),
		FinishedAt: f.finished.Format(time.RFC3339Nano),
	}}}, nil
}

func (f *fakeDocker) ContainerRemove(_ context.Context, id string, opts client.ContainerRemoveOptions) (client.ContainerRemoveResult, error) {
	if !opts.Force {
		panic("the executor must force removal")
	}
	f.mu.Lock()
	f.removed = append(f.removed, id)
	f.mu.Unlock()
	f.proc.kill()
	return client.ContainerRemoveResult{}, nil
}

func (f *fakeDocker) removedIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.removed...)
}

func testSpec() Spec {
	return Spec{
		Image:       "lightning-runtime-python:latest",
		Source:      "print('hi')",
		Stdin:       "input",
		MemoryBytes: 256 << 20,
		NanoCPUs:    1e9,
		PidsLimit:   128,
		Timeout:     5 * time.Second,
		MaxOutput:   1 << 10,
	}
}

func run(t *testing.T, ctx context.Context, f *fakeDocker, spec Spec) (*recordingSink, Result, error) {
	t.Helper()
	d := &Docker{api: f, log: slog.New(slog.DiscardHandler)}
	sink := &recordingSink{}
	res, err := d.Run(ctx, spec, sink)
	return sink, res, err
}

func TestRunReportsOutputAndExitCode(t *testing.T) {
	f := newFake(func(p *fakeProcess) int {
		_ = p.write(stdcopy.Stdout, "hello ")
		_ = p.write(stdcopy.Stderr, "warning\n")
		_ = p.write(stdcopy.Stdout, "world\n")
		return 3
	})
	sink, res, err := run(t, t.Context(), f, testSpec())
	if err != nil {
		t.Fatal(err)
	}
	if sink.started != 1 {
		t.Fatalf("Started called %d times", sink.started)
	}
	want := []chunk{{Stdout, "hello "}, {Stderr, "warning\n"}, {Stdout, "world\n"}}
	if !reflect.DeepEqual(sink.chunks, want) {
		t.Fatalf("chunks = %v, want %v", sink.chunks, want)
	}
	if res.ExitCode != 3 || res.TimedOut || res.OOMKilled || res.Truncated {
		t.Fatalf("result = %+v", res)
	}
	if got := f.removedIDs(); !reflect.DeepEqual(got, []string{"c1"}) {
		t.Fatalf("removed = %v", got)
	}
}

func TestRunTimeoutKillsProgram(t *testing.T) {
	f := newFake(func(p *fakeProcess) int {
		_ = p.write(stdcopy.Stdout, "working")
		return p.untilKilled()
	})
	spec := testSpec()
	spec.Timeout = 50 * time.Millisecond
	sink, res, err := run(t, t.Context(), f, spec)
	if err != nil {
		t.Fatal(err)
	}
	if !res.TimedOut || res.ExitCode != exitCodeKilled || res.Truncated {
		t.Fatalf("result = %+v", res)
	}
	if res.Duration < spec.Timeout {
		t.Fatalf("duration %v shorter than the timeout", res.Duration)
	}
	if sink.text(Stdout) != "working" {
		t.Fatalf("output before the timeout was lost: %q", sink.text(Stdout))
	}
	if f.killCount() != 1 || len(f.removedIDs()) != 1 {
		t.Fatalf("kills = %d, removed = %v", f.killCount(), f.removedIDs())
	}
}

func TestRunOutputLimitKillsProgram(t *testing.T) {
	f := newFake(func(p *fakeProcess) int {
		for {
			if err := p.write(stdcopy.Stdout, strings.Repeat("y", 100)); err != nil {
				return p.untilKilled()
			}
		}
	})
	sink, res, err := run(t, t.Context(), f, testSpec())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Truncated || res.TimedOut || res.ExitCode != exitCodeKilled {
		t.Fatalf("result = %+v", res)
	}
	if got := len(sink.text(Stdout)); got != testSpec().MaxOutput {
		t.Fatalf("output length = %d, want %d", got, testSpec().MaxOutput)
	}
	if f.killCount() != 1 || len(f.removedIDs()) != 1 {
		t.Fatalf("kills = %d, removed = %v", f.killCount(), f.removedIDs())
	}
}

// TestRunKillReasons checks that a program is killed once, and that the
// deadline passing while the engine is slow to kill it does not turn an output
// overflow into a timeout.
func TestRunKillReasons(t *testing.T) {
	flood := func(p *fakeProcess) int {
		for {
			if err := p.write(stdcopy.Stdout, strings.Repeat("y", 100)); err != nil {
				return p.untilKilled()
			}
		}
	}
	trickle := func(p *fakeProcess) int {
		for p.write(stdcopy.Stdout, strings.Repeat("y", 100)) == nil {
			select {
			case <-p.killed:
				return exitCodeKilled
			case <-time.After(10 * time.Millisecond):
			}
		}
		return p.untilKilled()
	}
	tests := []struct {
		name               string
		program            func(p *fakeProcess) int
		timeout            time.Duration
		timedOut, truncate bool
	}{
		// The output limit is hit at once; the deadline passes during the kill.
		{"output limit", flood, 200 * time.Millisecond, false, true},
		// The deadline passes first; the output limit is hit during the kill.
		{"timeout then output limit", trickle, 30 * time.Millisecond, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(tt.program)
			f.killDelay = 600 * time.Millisecond
			spec := testSpec()
			spec.Timeout = tt.timeout
			sink, res, err := run(t, t.Context(), f, spec)
			if err != nil {
				t.Fatal(err)
			}
			if res.TimedOut != tt.timedOut || res.Truncated != tt.truncate || res.ExitCode != exitCodeKilled {
				t.Fatalf("result = %+v", res)
			}
			if got := len(sink.text(Stdout)); got != spec.MaxOutput {
				t.Fatalf("output length = %d, want %d", got, spec.MaxOutput)
			}
			if n := f.killCount(); n != 1 {
				t.Fatalf("killed %d times", n)
			}
		})
	}
}

func TestRunLateExitIsNotATimeout(t *testing.T) {
	f := newFake(func(p *fakeProcess) int {
		_ = p.write(stdcopy.Stdout, "done\n")
		return 0
	})
	f.exitDelay = 500 * time.Millisecond
	spec := testSpec()
	spec.Timeout = 200 * time.Millisecond
	sink, res, err := run(t, t.Context(), f, spec)
	if err != nil {
		t.Fatal(err)
	}
	if res.TimedOut || res.ExitCode != 0 || res.Duration >= spec.Timeout || sink.text(Stdout) != "done\n" {
		t.Fatalf("result = %+v, stdout %q", res, sink.text(Stdout))
	}
}

func TestRunCancellationDuringKill(t *testing.T) {
	f := newFake(func(p *fakeProcess) int { return p.untilKilled() })
	f.killDelay = 500 * time.Millisecond
	spec := testSpec()
	spec.Timeout = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(100*time.Millisecond, cancel)
	if _, _, err := run(t, ctx, f, spec); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if got := f.removedIDs(); !reflect.DeepEqual(got, []string{"c1"}) {
		t.Fatalf("removed = %v", got)
	}
}

func TestRunCancellationRemovesContainer(t *testing.T) {
	f := newFake(func(p *fakeProcess) int { return p.untilKilled() })
	ctx, cancel := context.WithCancel(t.Context())
	d := &Docker{api: f, log: slog.New(slog.DiscardHandler)}
	sink := &cancelOnStart{cancel: cancel}

	start := time.Now()
	_, err := d.Run(ctx, testSpec(), sink)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("cancellation took %v", elapsed)
	}
	if got := f.removedIDs(); !reflect.DeepEqual(got, []string{"c1"}) {
		t.Fatalf("removed = %v", got)
	}
	select {
	case <-f.proc.killed:
	default:
		t.Fatal("program is still running")
	}
}

type cancelOnStart struct{ cancel context.CancelFunc }

func (c *cancelOnStart) Started()              { c.cancel() }
func (c *cancelOnStart) Output(Stream, []byte) {}

func TestRunReportsOOM(t *testing.T) {
	f := newFake(func(*fakeProcess) int { return exitCodeKilled })
	f.oom = true
	_, res, err := run(t, t.Context(), f, testSpec())
	if err != nil {
		t.Fatal(err)
	}
	if !res.OOMKilled || res.TimedOut || res.ExitCode != exitCodeKilled {
		t.Fatalf("result = %+v", res)
	}
}

func TestRunMissingImage(t *testing.T) {
	f := newFake(nil)
	f.createErr = cerrdefs.ErrNotFound.WithMessage("No such image")
	_, _, err := run(t, t.Context(), f, testSpec())
	if !errors.Is(err, ErrImageNotFound) || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v", err)
	}
	if len(f.removedIDs()) != 0 {
		t.Fatal("nothing was created, nothing should be removed")
	}
}

func TestRunStartFailureRemovesContainer(t *testing.T) {
	f := newFake(nil)
	f.startErr = errors.New("OCI runtime create failed")
	_, _, err := run(t, t.Context(), f, testSpec())
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v", err)
	}
	if got := f.removedIDs(); !reflect.DeepEqual(got, []string{"c1"}) {
		t.Fatalf("removed = %v", got)
	}
}

func TestRunWithCanceledContextDoesNothing(t *testing.T) {
	f := newFake(nil)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := run(t, ctx, f, testSpec()); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if len(f.created) != 0 {
		t.Fatal("a container was created for a canceled request")
	}
}

func TestCreateOptionsFollowTheRuntimeContract(t *testing.T) {
	d := &Docker{runtime: "runsc"}
	opts := d.createOptions(testSpec())

	if !strings.HasPrefix(opts.Name, "lightning-") {
		t.Errorf("name = %q", opts.Name)
	}
	cfg, hc := opts.Config, opts.HostConfig
	wantEnv := []string{
		"LIGHTNING_SOURCE_B64=" + base64.StdEncoding.EncodeToString([]byte("print('hi')")),
		"LIGHTNING_STDIN_B64=" + base64.StdEncoding.EncodeToString([]byte("input")),
	}
	checks := []struct {
		name      string
		got, want any
	}{
		{"image", cfg.Image, "lightning-runtime-python:latest"},
		{"cmd", cfg.Cmd, []string{"/opt/lightning/run"}},
		{"user", cfg.User, "10001:10001"},
		{"hostname", cfg.Hostname, "sandbox"},
		{"workdir", cfg.WorkingDir, "/workspace"},
		{"env", cfg.Env, wantEnv},
		{"labels", cfg.Labels, map[string]string{"lightning.sandbox": "true"}},
		{"init", *hc.Init, true},
		{"network", string(hc.NetworkMode), "none"},
		{"read-only", hc.ReadonlyRootfs, true},
		{"tmpfs", hc.Tmpfs, map[string]string{
			"/workspace": "rw,exec,nosuid,nodev,size=64m,uid=10001,gid=10001,mode=0700",
			"/tmp":       "rw,exec,nosuid,nodev,size=128m,uid=10001,gid=10001,mode=1777",
		}},
		{"cap-drop", hc.CapDrop, []string{"ALL"}},
		{"security-opt", hc.SecurityOpt, []string{"no-new-privileges"}},
		{"memory", hc.Memory, int64(256 << 20)},
		{"memory-swap", hc.MemorySwap, int64(256 << 20)},
		{"cpus", hc.NanoCPUs, int64(1e9)},
		{"pids-limit", *hc.PidsLimit, int64(128)},
		{"ulimits", hc.Ulimits, []*container.Ulimit{
			{Name: "nofile", Soft: 256, Hard: 256},
			{Name: "fsize", Soft: 16777216, Hard: 16777216},
		}},
		{"log-config", hc.LogConfig, container.LogConfig{Type: "json-file", Config: map[string]string{"max-size": "1m", "max-file": "1"}}},
		{"runtime", hc.Runtime, "runsc"},
		{"auto-remove", hc.AutoRemove, false},
		{"privileged", hc.Privileged, false},
	}
	for _, c := range checks {
		if !reflect.DeepEqual(c.got, c.want) {
			t.Errorf("%s = %#v, want %#v", c.name, c.got, c.want)
		}
	}
	if d.createOptions(testSpec()).Name == opts.Name {
		t.Error("container names must be unique")
	}
}

func TestRunTime(t *testing.T) {
	d, ok := runTime("2026-01-01T10:00:00.5Z", "2026-01-01T10:00:02Z")
	if !ok || d != 1500*time.Millisecond {
		t.Fatalf("runTime = %v, %v", d, ok)
	}
	for _, pair := range [][2]string{{"", ""}, {"0001-01-01T00:00:00Z", "bogus"}, {"2026-01-01T10:00:02Z", "2026-01-01T10:00:00Z"}} {
		if _, ok := runTime(pair[0], pair[1]); ok {
			t.Errorf("runTime(%q, %q) should fail", pair[0], pair[1])
		}
	}
}

type maintenanceFake struct {
	dockerAPI
	containers []container.Summary
	images     map[string]bool
	removed    []string
}

func (m *maintenanceFake) ContainerList(_ context.Context, opts client.ContainerListOptions) (client.ContainerListResult, error) {
	if !opts.All || !opts.Filters["label"]["lightning.sandbox=true"] || !opts.Filters["name"]["^lightning-"] {
		panic("stale sandboxes must be listed by label and name, including stopped containers")
	}
	return client.ContainerListResult{Items: m.containers}, nil
}

func (m *maintenanceFake) ContainerRemove(_ context.Context, id string, _ client.ContainerRemoveOptions) (client.ContainerRemoveResult, error) {
	m.removed = append(m.removed, id)
	return client.ContainerRemoveResult{}, nil
}

func (m *maintenanceFake) ImageInspect(_ context.Context, image string, _ ...client.ImageInspectOption) (client.ImageInspectResult, error) {
	if !m.images[image] {
		return client.ImageInspectResult{}, cerrdefs.ErrNotFound
	}
	return client.ImageInspectResult{}, nil
}

func TestRemoveStale(t *testing.T) {
	now := time.Now()
	m := &maintenanceFake{containers: []container.Summary{
		{ID: "stale", Created: now.Add(-time.Hour).Unix()},
		{ID: "running", Created: now.Add(-10 * time.Second).Unix()},
		{ID: "leaked", Created: now.Add(-2 * time.Minute).Unix()},
	}}
	d := &Docker{api: m, log: slog.New(slog.DiscardHandler)}
	n, err := d.RemoveStale(t.Context(), time.Minute)
	if err != nil || n != 2 || !reflect.DeepEqual(m.removed, []string{"stale", "leaked"}) {
		t.Fatalf("RemoveStale = %d, %v; removed %v", n, err, m.removed)
	}
}

func TestMissingImages(t *testing.T) {
	m := &maintenanceFake{images: map[string]bool{"present:latest": true}}
	d := &Docker{api: m, log: slog.New(slog.DiscardHandler)}
	missing, err := d.MissingImages(t.Context(), []string{"present:latest", "absent:latest"})
	if err != nil || !reflect.DeepEqual(missing, []string{"absent:latest"}) {
		t.Fatalf("MissingImages = %v, %v", missing, err)
	}
}
