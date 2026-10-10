# Security

Lightning executes code submitted by anonymous users. This document describes what it defends
against, how, and what it does not protect you from.

## Reporting a vulnerability

Please report vulnerabilities privately through
[GitHub private vulnerability reporting](https://github.com/camilbenameur/lightning/security/advisories/new)
(repository **Security** tab → **Report a vulnerability**), not in a public issue. Include the
affected version (`GET /api/v1/info`), reproduction steps and the impact you observed.

This is a personal project maintained on a best-effort basis: expect an acknowledgement within a
week. Fixes land on `main` and ship with the next release; only the latest release is supported.

Sandbox escapes, ways around the resource limits, reaching the network from a sandbox, reaching
the Docker API or the metrics endpoint from the Internet, and XSS in the web UI are all in scope.
Please do not test against instances you do not operate beyond what is needed to demonstrate an
issue, and never degrade them for other users.

## Threat model

Every submitted program is assumed hostile. It may try to:

- escape its container and take over the host (and with it every other sandbox);
- exhaust shared resources: CPU, memory, processes, disk, file descriptors, output volume, the
  run queue;
- use the server's network: scanning, spam, attacks on third parties, exfiltration;
- attack the people using the web UI, for example with output crafted as HTML or script.

Lightning stores nothing: code and output only exist for the duration of a run, and share links
carry the code in the URL fragment, which browsers never send to the server. The assets to protect
are the host, the availability of the service, and the confidentiality of in-flight runs.

## Defense in depth

### 1. Sandbox containers

Each run gets a fresh container that is killed and removed when the run ends, times out, exceeds
its output budget or the client disconnects. Leftovers, for example after a crash, are removed
when the API starts and by a sweep every minute. The settings are the runtime contract of
`runtimes/README.md`:

| Setting | Effect |
|---------|--------|
| `--network none` | No network interface besides loopback: no Internet, no access to other containers or the host. |
| `--read-only` + `tmpfs` `/workspace` (64 MB) and `/tmp` (128 MB), `nosuid,nodev` | Immutable image; scratch space is small, in memory and discarded. |
| `--user 10001:10001` | Unprivileged user inside the container. |
| `--cap-drop ALL`, `--security-opt no-new-privileges` | No capabilities, no privilege gain through setuid binaries or file capabilities. |
| Docker's default seccomp and AppArmor profiles | Left enabled: dangerous syscalls (`mount`, `bpf`, `kexec_load`, new namespaces...) are refused. |
| `--memory` = `--memory-swap` (256 MB; 512 MB for Go, Java, Rust) | Hard memory cap without swap; tmpfs pages count against it. OOM kills are reported as `oomKilled`. |
| `--cpus 1`, `--pids-limit 128` | Bounded CPU share; fork bombs hit the PID limit. |
| `--ulimit nofile=256`, `--ulimit fsize=16MB` | Bounded file descriptors and file sizes. |
| Wall-clock timeout (10 s, plus a compile allowance for compiled languages) | Runaway programs are killed (`timedOut`, exit code 137). |
| Output cap (64 KB, stdout and stderr combined) | The sandbox is killed once it is reached (`truncated`). Container logs are capped at 1 MB. |
| `--init` | A minimal init reaps zombies and forwards signals. |
| Input passed as environment variables | No bind mounts: no host path is ever visible inside a sandbox. |
| Optional `--runtime runsc` (gVisor) | Syscalls are served by a user-space kernel instead of the host kernel. |

### 2. Capacity controls in the API

- Request validation and size limits on code (64 KB), stdin (64 KB) and the request body.
- At most `LIGHTNING_MAX_CONCURRENT` sandboxes at once; extra runs wait in a bounded queue and get
  `503 server_busy` after `LIGHTNING_QUEUE_TIMEOUT`. Worst-case resource use is therefore bounded
  by configuration, whatever the load.
- A per-client-IP token bucket on the run endpoints (`429` with `Retry-After`). Behind Caddy the
  client address is the right-most `X-Forwarded-For` entry, which Caddy sets itself.

### 3. The API process

- Static binary on a distroless image: no shell, no package manager, runs as uid 65532.
- `read_only`, `cap_drop: ALL`, `no-new-privileges`, memory, CPU and PID limits.
- Attached only to `internal` networks: it cannot reach the Internet and has no published port.
- No Docker socket. It talks to the Docker daemon through the socket proxy.

### 4. Docker socket proxy

[`wollomatic/socket-proxy`](https://github.com/wollomatic/socket-proxy) holds the socket and
forwards an exact allowlist of method and path pairs: the calls the executor makes
(`internal/sandbox/docker.go`), namely container create, start, wait, logs, inspect, kill, remove
and list, image inspect and pull, `_ping` and `version`. Everything else is refused: `exec`,
`attach`, `archive` (copying files in or out), `update`, image push, tag, load and delete, `build`,
volumes, networks, secrets, swarm, `info`, `events`... Container creation requests that bind-mount
a host directory are refused as well. It accepts connections from the `api` service only, on an
internal network, and runs read-only without capabilities. The rules live in `compose.yaml`, and
every refused call is logged as `blocked request`.

### 5. Edge (Caddy)

- Automatic HTTPS with HSTS, HTTP/2 and HTTP/3.
- Only `/api/*` is proxied: `/healthz`, `/readyz` and the metrics listener (`:9090`) are
  unreachable from the Internet; `scripts/smoke.sh` checks this on every deployment.
- Request body limit and header/body read timeouts against slow or oversized uploads.
- Strict Content Security Policy: `script-src 'self'` (the Monaco editor is bundled, no CDN, no
  inline script), `connect-src 'self'`, `object-src 'none'`, `base-uri 'self'`,
  `form-action 'self'`, `frame-ancestors 'none'`. Program output is rendered as text.
- `X-Content-Type-Options: nosniff`, `Referrer-Policy`, `Permissions-Policy`,
  `Cross-Origin-Opener-Policy`, `Cross-Origin-Resource-Policy`, `X-Frame-Options`; no `Server`
  header.
- Runs as an unprivileged user with a read-only root filesystem and no capabilities.

### 6. Supply chain and delivery

- Images are built in CI from pinned base image versions, with SBOM and provenance attestations.
- Weekly rebuilds pick up base image security fixes; Dependabot tracks Go modules, npm packages,
  GitHub Actions and base images; CodeQL and golangci-lint run on every change.
- Workflows default to read-only tokens; only the release job can write packages. Deployment
  secrets live in a protected `production` environment, and the deploy job pins the server's SSH
  host key instead of trusting it on first use.

## Residual risks

These are known limits of the design. Weigh them before exposing an instance.

- **Shared kernel.** With the default runc runtime every sandbox shares the host kernel. Seccomp,
  AppArmor, dropped capabilities and an unprivileged user shrink the attack surface, but a kernel
  or container-runtime vulnerability reachable through the allowed syscalls can still lead to a
  full host compromise. **Run public instances under gVisor** (`LIGHTNING_SANDBOX_RUNTIME=runsc`,
  see `docs/DEPLOYMENT.md`), keep the kernel patched, and use a dedicated VM with nothing else of
  value on it.
- **The API remains a trusted component.** Apart from bind mounts, the socket proxy does not
  inspect container settings: whoever controls the API process can still create a privileged
  container and reach the host's devices, which is root on the host. The proxy keeps the socket
  out of the Internet-facing processes and removes most of the Docker API, which makes such an
  attack harder, not impossible. Mounting the socket `:ro` does not restrict API calls.
- **Docker access is root.** The daemon runs as root, and the deploy user (member of the `docker`
  group) is root-equivalent. Protect its SSH key; rootless Docker is possible with the caveats in
  `docs/DEPLOYMENT.md`.
- **Denial of service.** Rate limiting is per IP address. An attacker with many addresses (a
  botnet, or rotating through an IPv6 prefix) can keep the run queue full, and other users then get
  `503 server_busy`. Limits bound the damage to the host but not to availability; put a CDN or
  WAF in front, or add authentication, if that matters.
- **Abuse within the limits.** Each run may burn one CPU for the whole timeout (for example for
  cryptocurrency mining). The limits make this unprofitable, not impossible.
- **Side channels.** Sandboxes share CPU caches and cores with each other and the host;
  microarchitectural side channels are not mitigated beyond what the kernel and hypervisor do.
- **Operator visibility.** Source code and stdin travel in the sandbox container's environment:
  anyone with Docker access on the host can read in-flight submissions (`docker inspect`). The API
  and Caddy log request metadata such as client IP addresses, not code.

## Operator checklist

- [ ] gVisor installed and `LIGHTNING_SANDBOX_RUNTIME=runsc` set.
- [ ] Dedicated VM; SSH with keys only; `unattended-upgrades` and fail2ban enabled.
- [ ] Only ports 22, 80 and 443 reachable; nothing else published by Docker.
- [ ] `scripts/smoke.sh https://<domain>` passes after every deployment (the Deploy workflow runs it).
- [ ] Deployments follow `main` or a pinned release, so weekly base image rebuilds reach the server.
