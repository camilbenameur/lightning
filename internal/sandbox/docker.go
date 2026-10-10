package sandbox

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

const (
	// Label carried by every sandbox container.
	Label = "lightning.sandbox"
	// namePrefix starts the name of every container created by Docker. It
	// tells them apart from other labelled containers, such as those of the
	// runtime image tests.
	namePrefix = "lightning-"

	// exitCodeKilled is reported for runs killed on timeout (128 + SIGKILL).
	exitCodeKilled = 137
	// apiTimeout bounds Docker calls that must not depend on the caller's
	// context: creation, inspection, kill and removal.
	apiTimeout = 30 * time.Second
	// drainTimeout bounds how long the tail of the output is awaited after exit.
	drainTimeout = 2 * time.Second
	// killTimeout bounds how long a killed container may take to report its
	// exit. After a flood of output, the engine reports it only once its log
	// driver has caught up, which can take over 10 seconds.
	killTimeout = 30 * time.Second
)

// dockerAPI is the subset of the Docker Engine client used by this package.
type dockerAPI interface {
	Ping(context.Context, client.PingOptions) (client.PingResult, error)
	ContainerCreate(context.Context, client.ContainerCreateOptions) (client.ContainerCreateResult, error)
	ContainerStart(context.Context, string, client.ContainerStartOptions) (client.ContainerStartResult, error)
	ContainerWait(context.Context, string, client.ContainerWaitOptions) client.ContainerWaitResult
	ContainerLogs(context.Context, string, client.ContainerLogsOptions) (client.ContainerLogsResult, error)
	ContainerKill(context.Context, string, client.ContainerKillOptions) (client.ContainerKillResult, error)
	ContainerInspect(context.Context, string, client.ContainerInspectOptions) (client.ContainerInspectResult, error)
	ContainerRemove(context.Context, string, client.ContainerRemoveOptions) (client.ContainerRemoveResult, error)
	ContainerList(context.Context, client.ContainerListOptions) (client.ContainerListResult, error)
	ImageInspect(context.Context, string, ...client.ImageInspectOption) (client.ImageInspectResult, error)
	ImagePull(context.Context, string, client.ImagePullOptions) (client.ImagePullResponse, error)
}

// Docker is an Executor that runs every program in a fresh, locked-down
// container on a Docker Engine.
type Docker struct {
	api     dockerAPI
	runtime string
	log     *slog.Logger
}

// NewDocker returns an executor backed by cli. runtime optionally selects the
// OCI runtime of sandbox containers, such as gVisor's "runsc".
func NewDocker(cli *client.Client, runtime string, log *slog.Logger) *Docker {
	return &Docker{api: cli, runtime: runtime, log: log}
}

// Run implements Executor.
func (d *Docker) Run(ctx context.Context, spec Spec, sink Sink) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	// Creation must not be interrupted halfway: the container could be created
	// without its ID ever reaching us, leaving it behind.
	detached := context.WithoutCancel(ctx)
	createCtx, cancel := context.WithTimeout(detached, apiTimeout)
	created, err := d.api.ContainerCreate(createCtx, d.createOptions(spec))
	cancel()
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return Result{}, fmt.Errorf("%w: %s", ErrImageNotFound, spec.Image)
		}
		return Result{}, fmt.Errorf("%w: create container: %w", ErrUnavailable, err)
	}
	// Deferred so that it also runs on panics.
	defer d.remove(detached, created.ID)

	return d.execute(ctx, created.ID, spec, sink)
}

func (d *Docker) execute(ctx context.Context, id string, spec Spec, sink Sink) (Result, error) {
	detached := context.WithoutCancel(ctx)

	// Subscribe to the exit before starting, so that a fast exit is not missed.
	waitCtx, stopWaiting := context.WithCancel(detached)
	defer stopWaiting()
	exit := d.waitExit(waitCtx, id)

	if _, err := d.api.ContainerStart(ctx, id, client.ContainerStartOptions{}); err != nil {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		return Result{}, fmt.Errorf("%w: start container: %w", ErrUnavailable, err)
	}
	started := time.Now()
	sink.Started()

	out := newOutput(sink, spec.MaxOutput)
	logsCtx, stopLogs := context.WithCancel(detached)
	copied := make(chan error, 1)
	var copying sync.WaitGroup
	copying.Go(func() { copied <- d.copyLogs(logsCtx, id, out) })
	// The sink must not be used once Run has returned.
	defer func() {
		stopLogs()
		copying.Wait()
	}()

	var (
		res      Result
		logs     = (<-chan error)(copied)
		timeout  = time.NewTimer(spec.Timeout)
		killWait <-chan time.Time
		killing  sync.WaitGroup
	)
	defer timeout.Stop()
	defer killing.Wait()
	// kill stops the program once, whatever the reason. The engine answers a
	// SIGKILL only when the exit has been processed, which can take seconds:
	// the loop keeps watching the exit, the client and the deadline meanwhile.
	kill := func() {
		if killWait != nil {
			return
		}
		killWait = time.After(killTimeout)
		killing.Go(func() { d.kill(detached, id) })
	}
	for exited := false; !exited; {
		select {
		case st := <-exit:
			if st.err != nil {
				if ctx.Err() != nil {
					return Result{}, ctx.Err()
				}
				return Result{}, fmt.Errorf("%w: wait for container: %w", ErrUnavailable, st.err)
			}
			res.ExitCode = st.code
			exited = true
		case err := <-logs:
			logs = nil
			if d.logsDone(id, err) {
				res.Truncated = true
				// Killed for its output: the deadline passing while the
				// program dies does not make it a timeout.
				timeout.Stop()
				kill()
			}
		case <-timeout.C:
			res.TimedOut = true
			kill()
		case <-killWait:
			return Result{}, fmt.Errorf("%w: container did not exit after being killed", ErrUnavailable)
		case <-ctx.Done():
			return Result{}, ctx.Err() // the deferred removal kills the container
		}
	}
	res.Duration = time.Since(started)

	if logs != nil {
		select {
		case err := <-logs:
			res.Truncated = d.logsDone(id, err)
		case <-time.After(drainTimeout):
			d.log.Warn("sandbox output did not end after exit", "container", id)
		}
	}
	stopLogs()
	copying.Wait()
	out.flush()

	d.inspect(detached, id, &res)
	// The deadline can pass while the exit of a program that ended in time is
	// still on its way; the engine's record of the run time tells them apart.
	if res.TimedOut && res.Duration < spec.Timeout {
		res.TimedOut = false
	}
	if res.TimedOut {
		res.ExitCode = exitCodeKilled
	}
	return res, nil
}

// inspect completes res with what the engine recorded about the run: whether
// it ran out of memory, and its exact run time.
func (d *Docker) inspect(ctx context.Context, id string, res *Result) {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()
	info, err := d.api.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		d.log.Warn("inspect sandbox container", "container", id, "error", err)
		return
	}
	if st := info.Container.State; st != nil {
		res.OOMKilled = st.OOMKilled
		if runtime, ok := runTime(st.StartedAt, st.FinishedAt); ok {
			res.Duration = runtime
		}
	}
}

type exitStatus struct {
	code int
	err  error
}

// waitExit subscribes to the next exit of the container. The channel yields
// exactly one value; cancel ctx to abandon the wait.
func (d *Docker) waitExit(ctx context.Context, id string) <-chan exitStatus {
	w := d.api.ContainerWait(ctx, id, client.ContainerWaitOptions{Condition: container.WaitConditionNextExit})
	ch := make(chan exitStatus, 1)
	go func() {
		select {
		case r := <-w.Result:
			ch <- exitStatus{code: int(r.StatusCode)}
		case err := <-w.Error:
			ch <- exitStatus{err: err}
		}
	}()
	return ch
}

// logsDone handles the end of output copying and reports whether the output
// limit was hit.
func (d *Docker) logsDone(id string, err error) bool {
	if errors.Is(err, errOutputLimit) {
		return true
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		d.log.Warn("read sandbox output", "container", id, "error", err)
	}
	return false
}

func (d *Docker) copyLogs(ctx context.Context, id string, out *output) error {
	logs, err := d.api.ContainerLogs(ctx, id, client.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err != nil {
		return err
	}
	defer logs.Close()
	_, err = stdcopy.StdCopy(out.writer(Stdout), out.writer(Stderr), logs)
	return err
}

func (d *Docker) kill(ctx context.Context, id string) {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()
	_, err := d.api.ContainerKill(ctx, id, client.ContainerKillOptions{Signal: "KILL"})
	// A conflict means the container is no longer running.
	if err != nil && !cerrdefs.IsConflict(err) && !cerrdefs.IsNotFound(err) {
		d.log.Warn("kill sandbox container", "container", id, "error", err)
	}
}

func (d *Docker) remove(ctx context.Context, id string) {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()
	_, err := d.api.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
	if err != nil && !cerrdefs.IsNotFound(err) {
		d.log.Error("remove sandbox container", "container", id, "error", err)
	}
}

// runTime returns the time between the container's start and exit as
// recorded by the engine, which excludes API round-trips.
func runTime(startedAt, finishedAt string) (time.Duration, bool) {
	start, err1 := time.Parse(time.RFC3339Nano, startedAt)
	end, err2 := time.Parse(time.RFC3339Nano, finishedAt)
	if err1 != nil || err2 != nil || end.Before(start) {
		return 0, false
	}
	return end.Sub(start), true
}

// createOptions translates spec into the canonical sandbox container settings
// of the runtime contract (see runtimes/README.md). The container is removed
// explicitly rather than with AutoRemove, so that it can be inspected after exit.
func (d *Docker) createOptions(spec Spec) client.ContainerCreateOptions {
	initProcess := true
	pids := spec.PidsLimit
	return client.ContainerCreateOptions{
		Name: namePrefix + strings.ToLower(rand.Text()),
		Config: &container.Config{
			Image:      spec.Image,
			Cmd:        []string{"/opt/lightning/run"},
			User:       "10001:10001",
			Hostname:   "sandbox",
			WorkingDir: "/workspace",
			Env: []string{
				"LIGHTNING_SOURCE_B64=" + base64.StdEncoding.EncodeToString([]byte(spec.Source)),
				"LIGHTNING_STDIN_B64=" + base64.StdEncoding.EncodeToString([]byte(spec.Stdin)),
			},
			Labels: map[string]string{Label: "true"},
		},
		HostConfig: &container.HostConfig{
			Init:           &initProcess,
			NetworkMode:    "none",
			ReadonlyRootfs: true,
			Tmpfs: map[string]string{
				"/workspace": "rw,exec,nosuid,nodev,size=64m,uid=10001,gid=10001,mode=0700",
				"/tmp":       "rw,exec,nosuid,nodev,size=128m,uid=10001,gid=10001,mode=1777",
			},
			CapDrop:     []string{"ALL"},
			SecurityOpt: []string{"no-new-privileges"},
			LogConfig: container.LogConfig{
				Type:   "json-file",
				Config: map[string]string{"max-size": "1m", "max-file": "1"},
			},
			Runtime: d.runtime,
			Resources: container.Resources{
				Memory:     spec.MemoryBytes,
				MemorySwap: spec.MemoryBytes,
				NanoCPUs:   spec.NanoCPUs,
				PidsLimit:  &pids,
				Ulimits: []*container.Ulimit{
					{Name: "nofile", Soft: 256, Hard: 256},
					{Name: "fsize", Soft: 16 << 20, Hard: 16 << 20},
				},
			},
		},
	}
}

// Ping checks that the Docker Engine is reachable.
func (d *Docker) Ping(ctx context.Context) error {
	_, err := d.api.Ping(ctx, client.PingOptions{})
	return err
}

// RemoveStale removes the sandbox containers created more than maxAge ago and
// returns how many there were. A sandbox outlives its run only when the
// process that started it crashed or failed to remove it; going by age leaves
// alone the sandboxes of other processes sharing the engine.
func (d *Docker) RemoveStale(ctx context.Context, maxAge time.Duration) (int, error) {
	list, err := d.api.ContainerList(ctx, client.ContainerListOptions{
		All:     true,
		Filters: make(client.Filters).Add("label", Label+"=true").Add("name", "^"+namePrefix),
	})
	if err != nil {
		return 0, err
	}
	cutoff := time.Now().Add(-maxAge)
	removed := 0
	for _, c := range list.Items {
		if time.Unix(c.Created, 0).Before(cutoff) {
			d.remove(ctx, c.ID)
			removed++
		}
	}
	return removed, nil
}

// MissingImages returns the images that are not present on the engine.
func (d *Docker) MissingImages(ctx context.Context, images []string) ([]string, error) {
	var missing []string
	for _, image := range images {
		_, err := d.api.ImageInspect(ctx, image)
		switch {
		case cerrdefs.IsNotFound(err):
			missing = append(missing, image)
		case err != nil:
			return nil, err
		}
	}
	return missing, nil
}

// Pull downloads an image and waits for the pull to complete.
func (d *Docker) Pull(ctx context.Context, image string) error {
	resp, err := d.api.ImagePull(ctx, image, client.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer resp.Close()
	return resp.Wait(ctx)
}
