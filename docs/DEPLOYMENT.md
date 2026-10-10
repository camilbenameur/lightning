# Deploying Lightning

Lightning runs on a single Linux server with Docker. This guide takes a fresh VPS to a public
HTTPS instance, then sets up continuous deployment from GitHub Actions. Every command is meant to
be copied as is; replace `lightning.example.com` with your domain.

- [Architecture](#architecture)
- [Requirements](#requirements)
- [First-time server setup](#first-time-server-setup)
- [Continuous deployment with GitHub Actions](#continuous-deployment-with-github-actions)
- [Updating and rolling back](#updating-and-rolling-back)
- [Configuration](#configuration)
- [Hardening](#hardening)
- [Observability](#observability)
- [Troubleshooting](#troubleshooting)
- [Running from source](#running-from-source)

## Architecture

```
Internet ──80/443──▶ web (Caddy) ──/api/*──▶ api (Go) ──▶ docker-proxy ──▶ Docker daemon ──▶ sandbox containers
                     TLS, SPA,               :8080         :2375, filters   (host)            one per run,
                     security headers        :9090 metrics  the Docker API                    --network none
```

`compose.yaml` defines three services and three networks:

| Service        | Image                                         | Networks            | Notes |
|----------------|-----------------------------------------------|---------------------|-------|
| `web`          | `ghcr.io/camilbenameur/lightning-web`         | `edge`, `app`       | Only service with published ports (80, 443/tcp, 443/udp). Obtains and renews certificates. Proxies `/api/*` only. |
| `api`          | `ghcr.io/camilbenameur/lightning-api`         | `app`, `docker-api` | No published ports, no Internet access, no Docker socket. |
| `docker-proxy` | `wollomatic/socket-proxy`                     | `docker-api`        | Mounts the Docker socket. Lets through only the `api` service and only the Docker API calls it makes. |

`app` and `docker-api` are `internal` networks: containers attached only to them cannot reach the
Internet. Sandbox containers are created by the host daemon with `--network none`, so they join no
network at all. The only state is the `caddy_data` volume (ACME account and certificates).

Images are published by the `Release` workflow for every push to `main` (`latest`, `sha-<commit>`)
and every `v*` tag (`1.2.3`, `1.2`). `lightning-api` and `lightning-web` are multi-arch
(amd64, arm64); the eleven `lightning-runtime-<language>` images are **amd64 only**.

## Requirements

- **Server**: x86_64, 2 vCPU and 4 GB RAM recommended (4 concurrent sandboxes of up to 512 MB each).
  1 vCPU / 2 GB works with `LIGHTNING_MAX_CONCURRENT=1` or `2`.
- **Disk**: 25 GB or more. The eleven runtime images take about 4 GB (the Rust, Java and Go
  toolchains are most of it) and a deployment briefly holds two versions of the images that changed.
- **OS**: a current Linux distribution with cgroup v2, for example Ubuntu 24.04 LTS or Debian 12/13.
- **Docker Engine** 25+ with the Compose plugin 2.24+.
- **DNS**: an `A` record (and `AAAA` if the server has IPv6) for your domain pointing at the server.
- **Ports** 80/tcp, 443/tcp and 443/udp open to the Internet, including in your provider's
  firewall or security group. Port 80 is needed for the ACME HTTP challenge and redirects.

## First-time server setup

Run these steps as a sudo-capable user on the server. They are deliberately not a script: read
each one before running it.

### 1. Update the system and enable automatic security updates

```sh
sudo apt-get update && sudo apt-get -y upgrade
sudo apt-get install -y ca-certificates curl jq unattended-upgrades
sudo dpkg-reconfigure -plow unattended-upgrades
```

`jq` is optional; `scripts/smoke.sh` uses it for stricter JSON checks when it is installed.

### 2. Install Docker Engine

From Docker's official apt repository (Ubuntu shown; for Debian replace `ubuntu` with `debian`
in both URLs):

```sh
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "${UBUNTU_CODENAME:-$VERSION_CODENAME}") stable" \
  | sudo tee /etc/apt/sources.list.d/docker.list >/dev/null
sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
docker --version && docker compose version
docker info --format '{{.CgroupVersion}} {{.MemoryLimit}} {{.SwapLimit}} {{.PidsLimit}}'   # expect: 2 true true true
```

The sandbox limits depend on the last command: with `SwapLimit` false, a program that exceeds
its memory limit is swapped out instead of being OOM-killed, so it runs slowly until the timeout.

### 3. Firewall

```sh
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 443/udp
sudo ufw enable
```

Docker publishes container ports through its own iptables rules, which take precedence over ufw:
ports published by a container are reachable even without a ufw rule. The stack publishes only
80 and 443; do not add `ports:` to other services.

### 4. Create a deploy user

The deploy user runs Docker commands, locally and from GitHub Actions.

```sh
sudo adduser --disabled-password --gecos "" deploy
sudo usermod -aG docker deploy
sudo install -d -o deploy -g deploy -m 700 /home/deploy/.ssh
```

Membership of the `docker` group is equivalent to root on this machine: protect this account's
SSH keys accordingly (see [Hardening](#hardening)).

### 5. Install the stack

The server only needs the compose file, the environment file and two scripts, not the source:

```sh
sudo install -d -o deploy -g deploy /opt/lightning
sudo -iu deploy
cd /opt/lightning
base=https://raw.githubusercontent.com/camilbenameur/lightning/main
curl -fsSLO "$base/compose.yaml"
curl -fsSLO "$base/.env.example"
curl -fsSL --create-dirs -o scripts/deploy.sh "$base/scripts/deploy.sh"
curl -fsSL --create-dirs -o scripts/smoke.sh "$base/scripts/smoke.sh"
cp .env.example .env
chmod 600 .env
```

Edit `.env` and set at least your domain (every variable is documented in the file):

```sh
sed -i 's/^DOMAIN=.*/DOMAIN=lightning.example.com/' .env
sed -i 's/^ACME_EMAIL=.*/ACME_EMAIL=you@example.com/' .env   # optional
```

### 6. Make the images pullable

GHCR packages published from a personal account start **private**. After the first `Release`
workflow run, open each package on `https://github.com/camilbenameur?tab=packages`
(`lightning-api`, `lightning-web` and the eleven `lightning-runtime-*`), then
**Package settings → Change visibility → Public**.

Keeping them private also works if the server logs in to GHCR with a personal access token
limited to `read:packages` (`docker login ghcr.io -u <user>`) and every deployment goes through
`scripts/deploy.sh`, which pre-pulls the runtime images with those credentials. In that case also
set `LIGHTNING_PULL_IMAGES=false`: the API's own pulls go through the Docker API without them.

### 7. Start

```sh
bash scripts/deploy.sh
```

The script pulls all images (runtimes included, a few minutes on the first run), starts the
stack, waits until every service reports healthy and prints `docker compose ps`. Caddy obtains the
certificate within seconds once DNS and ports are right.

A plain `docker compose up -d --wait` works too: with `LIGHTNING_PULL_IMAGES=true` (the default)
the API pulls missing runtime images in the background when it starts, and runs of a language
answer `503 sandbox_unavailable` until its image has arrived.

### 8. Verify

From the server or from any machine with `bash` and `curl`:

```sh
bash scripts/smoke.sh https://lightning.example.com
```

It checks the security headers, that `/healthz`, `/readyz` and `/metrics` are not exposed, runs a
hello world in every enabled language, reads stdin, streams NDJSON output and checks the error
envelopes. It prints one line per check and exits non-zero on any failure.

## Continuous deployment with GitHub Actions

Pipeline: `CI` (every push and pull request) · `Release` (push to `main`, `v*` tags, weekly
rebuild on fresh base images) → `Deploy` (after every successful release of `main`, or manually).

`Deploy` connects to the server over SSH, uploads `compose.yaml`, `.env.example` and the scripts
of the released commit, runs `scripts/deploy.sh sha-<commit>` and then `scripts/smoke.sh` against
`https://$DOMAIN` from the runner. Until the secrets below exist it succeeds without doing anything.

### 1. Create a dedicated SSH key

On your workstation:

```sh
ssh-keygen -t ed25519 -N "" -C "github-actions-deploy" -f lightning-deploy
```

Authorize it for the deploy user on the server:

```sh
sudo -iu deploy sh -c 'cat >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys' < lightning-deploy.pub
```

Record the server's host key. Compare the fingerprint printed by the first command (run on your
workstation) with the second (run on the server) before trusting it:

```sh
ssh-keyscan -p 22 -t ed25519 lightning.example.com | tee known_hosts | ssh-keygen -lf -
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
```

### 2. Configure the `production` environment

Repository **Settings → Environments → New environment** named `production`. Optionally restrict
it to the `main` branch and add required reviewers. Then add:

| Kind     | Name                 | Value |
|----------|----------------------|-------|
| Secret   | `DEPLOY_HOST`        | Server hostname or IP address |
| Secret   | `DEPLOY_USER`        | `deploy` |
| Secret   | `DEPLOY_SSH_KEY`     | Content of `lightning-deploy` (the private key) |
| Secret   | `DEPLOY_KNOWN_HOSTS` | Content of `known_hosts` from the previous step |
| Variable | `DOMAIN`             | `lightning.example.com` (used for the smoke test and the environment URL) |
| Variable | `DEPLOY_PATH`        | Optional, default `/opt/lightning` |
| Variable | `DEPLOY_PORT`        | Optional, default `22` (with another port, scan it with `ssh-keyscan -p <port>`) |

With the GitHub CLI:

```sh
gh secret set DEPLOY_HOST --env production --body "lightning.example.com"
gh secret set DEPLOY_USER --env production --body "deploy"
gh secret set DEPLOY_SSH_KEY --env production < lightning-deploy
gh secret set DEPLOY_KNOWN_HOSTS --env production < known_hosts
gh variable set DOMAIN --env production --body "lightning.example.com"
```

Delete the local private key once it is stored in GitHub.

### 3. Deploy on demand

**Actions → Deploy → Run workflow**, with an image tag (`latest`, `sha-1a2b3c4`, `1.2.0`), or:

```sh
gh workflow run deploy.yml -f version=sha-1a2b3c4
```

## Updating and rolling back

`LIGHTNING_VERSION` in `.env` selects the image tag of the API, the web UI and, unless
`LIGHTNING_RUNTIME_TAG` is set, the runtime images, so the whole stack moves together:

| Tag           | Meaning |
|---------------|---------|
| `latest`      | Tip of `main` at the last release |
| `sha-1a2b3c4` | The build of that commit (what continuous deployment pins) |
| `1.2.0`, `1.2`| A `v1.2.0` git tag |

`scripts/deploy.sh <tag>` writes the tag into `.env` and deploys it; without an argument it
redeploys the pinned tag (pulling it again, which picks up a new `latest`):

```sh
cd /opt/lightning
bash scripts/deploy.sh               # update to the newest build of the pinned tag
bash scripts/deploy.sh 1.2.0         # move to a release
bash scripts/deploy.sh sha-1a2b3c4   # roll back to a known-good build
```

After a successful deployment the script deletes the other tags of Lightning's images and their
dangling layers; a rollback pulls the older version again. Available tags are listed on the
package pages on GHCR.

## Configuration

Everything is configured in `.env`; `.env.example` documents each variable and its default. The
most relevant ones:

| Variable | Default | Purpose |
|----------|---------|---------|
| `DOMAIN` | `localhost` | Site address. A hostname gets automatic HTTPS; `:80` serves plain HTTP. |
| `ACME_EMAIL` | empty | ACME account contact (optional). |
| `LIGHTNING_VERSION` | `latest` | Image tag of the whole stack. |
| `LIGHTNING_LANGUAGES` | all | Comma-separated language ids to enable. |
| `LIGHTNING_SANDBOX_RUNTIME` | runc | `runsc` to run sandboxes under gVisor. |
| `LIGHTNING_RUN_TIMEOUT` | `10s` | Wall-clock limit per run (compiled languages get a compile allowance on top). |
| `LIGHTNING_MEMORY_MB` | `256` | Memory per sandbox (Go, Java and Rust use 512). |
| `LIGHTNING_MAX_CONCURRENT` | `4` | Sandboxes running at once. |
| `LIGHTNING_QUEUE_TIMEOUT` | `20s` | How long a run waits for a slot before `503 server_busy`. |
| `LIGHTNING_RATE_LIMIT` / `_BURST` | `30` / `10` | Runs per minute and burst, per client IP. |

Apply changes with `docker compose up -d` (or `bash scripts/deploy.sh`). The API refuses to start
on an invalid value and names the variable in its log (`docker compose logs api`).

## Hardening

The default configuration is already locked down (see [SECURITY.md](../SECURITY.md)). For a
public instance, also consider the following.

### Run sandboxes under gVisor (recommended)

With the default runc runtime, sandboxes share the host kernel; a kernel vulnerability reachable
from inside a container can lead to a host compromise. [gVisor](https://gvisor.dev) runs each
sandbox against a user-space kernel, which removes most of that attack surface for a moderate
startup and syscall overhead.

```sh
curl -fsSL https://gvisor.dev/archive.key | sudo gpg --dearmor -o /usr/share/keyrings/gvisor-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/gvisor-archive-keyring.gpg] https://storage.googleapis.com/gvisor/releases release main" \
  | sudo tee /etc/apt/sources.list.d/gvisor.list >/dev/null
sudo apt-get update && sudo apt-get install -y runsc
sudo runsc install                 # registers the runtime in /etc/docker/daemon.json
sudo systemctl reload docker
docker run --rm --runtime=runsc hello-world
```

Then enable it and redeploy:

```sh
cd /opt/lightning
sed -i 's/^LIGHTNING_SANDBOX_RUNTIME=.*/LIGHTNING_SANDBOX_RUNTIME=runsc/' .env
bash scripts/deploy.sh && bash scripts/smoke.sh https://lightning.example.com
```

Expect slower compile-heavy languages; raise `LIGHTNING_RUN_TIMEOUT` if they get close to it.

### Rootless Docker

Running the daemon rootless means a container escape lands in an unprivileged account instead of
root. It is workable but has sharp edges:

- Point the proxy at the rootless socket: `DOCKER_SOCKET=/run/user/<uid>/docker.sock` in `.env`.
- Binding ports 80/443 needs `net.ipv4.ip_unprivileged_port_start=80` (set it in `/etc/sysctl.d/`).
- Memory, CPU and PID limits only apply if systemd delegates the `cpu cpuset io memory pids`
  cgroup v2 controllers to the user (see Docker's rootless documentation). Without delegation the
  sandbox limits are silently ignored: check with `docker info` (no "No cpu/memory limit support"
  warnings) before exposing the instance.

### Host

- SSH: keys only (`PasswordAuthentication no`, `PermitRootLogin no` in `/etc/ssh/sshd_config`).
- `sudo apt-get install -y fail2ban`: its default `sshd` jail throttles brute-force attempts.
- Keep `unattended-upgrades` enabled and reboot for kernel updates, which matter most with runc.
- Give the instance its own VM: nothing else of value should live on a host that runs code from
  strangers.
- Optionally restrict the deploy key in `~deploy/.ssh/authorized_keys` with
  `from="<addresses>"` if you deploy from fixed addresses (GitHub-hosted runners have no fixed IPs).

## Observability

```sh
cd /opt/lightning
docker compose ps                                  # state and health of each service
docker compose logs -f --tail 100 api              # JSON logs (slog); also: web, docker-proxy
docker compose logs --no-log-prefix api | jq -c 'select(.level == "ERROR")'
docker compose logs --no-log-prefix web | jq -c 'select(.logger | startswith("http.log.access"))' | tail
docker ps --filter label=lightning.sandbox=true    # sandboxes running right now
docker stats --no-stream                           # CPU and memory per container
```

Health endpoints and metrics are only reachable inside the `app` network. Query them through the
`web` container, which has `wget`:

```sh
docker compose exec web wget -qO- http://api:8080/healthz    # liveness (used by the healthcheck)
docker compose exec web wget -qO- http://api:8080/readyz     # 200 when the Docker daemon answers
docker compose exec web wget -qO- http://api:9090/metrics | grep -v '^#'
```

To scrape the Prometheus metrics, run Prometheus in a container attached to the `lightning_app`
network with the target `api:9090`. Never publish port 9090 on the host.

Container logs rotate at 3 × 10 MB per service; sandbox logs are capped at 1 MB and deleted with
their container.

## Troubleshooting

**No certificate / TLS errors.** `docker compose logs web | grep -iE 'acme|tls|challenge'`. Check
that `dig +short lightning.example.com` returns the server's address (both A and AAAA if you
have an AAAA record), that ports 80 and 443 are open in the provider firewall, and that `DOMAIN`
has no scheme or path. Repeated failures can hit Let's Encrypt rate limits: fix the cause, then
wait before retrying.

**`api` unhealthy or restarting.** `docker compose logs api` shows invalid configuration
(`LIGHTNING_*: must be ...`) or Docker connection errors. Check that `docker-proxy` is healthy and
that `DOCKER_SOCKET` points at the daemon socket.

**Runs fail with `503 sandbox_unavailable`.** A runtime image is missing or the daemon is
unreachable. `docker images | grep lightning-runtime` should list one image per enabled language
with the tag in use; `bash scripts/deploy.sh` pulls them. With private GHCR packages, see
[step 6](#6-make-the-images-pullable).

**Runs fail with `503 server_busy`.** All sandbox slots were busy for `LIGHTNING_QUEUE_TIMEOUT`.
Raise `LIGHTNING_MAX_CONCURRENT` if the server has headroom (`docker stats`), or add capacity.

**`429 rate_limited`.** A client exceeded `LIGHTNING_RATE_LIMIT`; the `Retry-After` header says
when to retry. The API relies on Caddy's `X-Forwarded-For`; do not put another proxy in front
without adjusting this.

**Docker socket permission errors on SELinux hosts** (Fedora, RHEL): the proxy cannot read the
socket. Add `security_opt: ["label=disable"]` to `docker-proxy` in a `compose.override.yaml`
next to `compose.yaml` (Compose loads it automatically).

**Runs fail and `docker compose logs docker-proxy` shows `blocked request`.** The API made a
Docker API call that the socket proxy's allowlist (`command:` of `docker-proxy` in
`compose.yaml`) does not permit, typically because `compose.yaml` is older than the API image.
Deploy the `compose.yaml` of the same release; the Deploy workflow uploads it on every run.

**Leftover sandbox containers**, for example after a crash, are removed when the API starts and
by a sweep every minute (those older than the longest possible run). To remove them by hand:
`docker rm -f $(docker ps -aq --filter label=lightning.sandbox=true)`.

**Disk filling up.** `docker system df`. Deployments delete superseded Lightning images; other
unused images can be removed with `docker image prune`.

## Running from source

For development or to try the stack locally without GHCR, `compose.local.yaml` builds the images
from source and serves plain HTTP on <http://localhost:8088>:

```sh
make runtimes LANGUAGES="python javascript"     # build the sandbox images you want
LIGHTNING_LANGUAGES=python,javascript make up    # build lightning-api/web:dev and start
make smoke                                       # scripts/smoke.sh http://localhost:8088
make down
```
