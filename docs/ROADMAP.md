# bianpai Roadmap

bianpai (编排) is a Docker Compose-style orchestrator that drives multiple container
CLIs through a single [`Backend`](../internal/backend/backend.go) interface. The compose
file is parsed once ([`internal/compose`](../internal/compose)), translated into
`RunRequest` / `BuildRequest` values, and each backend renders those into an argv for
its own CLI. Adding a backend is therefore additive: implement `Backend`, wire it into
backend selection, no changes to the compose layer.

## Current state (shipped)

- Cobra CLI: `up`, `down`, `ps`, `logs`, `build`, `pull`, `exec`, `version`.
- Compose discovery + partial loader (services, networks, volumes, common service keys).
- Project/container/network/volume naming and label-based grouping
  (`com.bianpai.project`, `com.bianpai.service`).
- WSLC backend (shells out to `wslc`).
- **Apple `container` backend** (`--backend container`) — Stage 1, see below.
- Capability warnings for keys a backend can't honor (restart, privileged,
  `network_mode`, `extra_hosts`, healthcheck, deploy, profiles) plus a macOS-26
  networking gate.
- Usecase corpus under [`usecases/`](../usecases) gated by `TestUsecaseStacksLoad`
  and the `make smoke` end-to-end argv check.

## Stage 1 — Apple `container` backend ✅ shipped

Goal (partially met): `bianpai --backend container up` runs supported compose projects on
Apple's `container` CLI on macOS. It is not at WSLC parity because Apple `container` does
not provide Docker/Podman-style Compose service discovery by default.

### Command mapping (`container` CLI)

| Backend method       | `container` invocation                                             |
| -------------------- | ----------------------------------------------------------------- |
| `Check`              | `container --version` (or `container system status`)              |
| `Build`              | `container build -t <tag> -f <file> --build-arg .. --target .. [--no-cache] [--pull] <ctx>` |
| `Pull`               | `container image pull <image>`  *(note: `image pull`, not `pull`)* |
| `Run`                | `container run [-d] [--rm] --name .. --env .. --env-file .. --label .. -p .. -v .. --network .. -w .. -u .. -m .. -c .. [--entrypoint] [-i] [-t] <image> <cmd>` |
| `Stop`               | `container stop <name>`                                            |
| `Remove`             | `container delete <name>`  *(alias `rm`)*                          |
| `Exec`               | `container exec <name> <cmd...>`                                   |
| `Logs`               | `container logs [-f] <name>`                                       |
| `List`               | `container list --all --format json` **+ client-side label filter** |
| `CreateNetwork`      | `container network create --label .. <name>`  *(macOS 26+)*        |
| `RemoveNetwork`      | `container network delete <name>`                                 |
| `CreateVolume`       | `container volume create --label .. <name>`                       |
| `RemoveVolume`       | `container volume delete <name>`                                  |

### The one interface divergence

`container list` has **no `--filter`** flag. WSLC filters by
`--filter label=com.bianpai.project=<project>` server-side; the container backend must
fetch `container list --all --format json`, decode it, keep only rows whose labels carry
the project label, then render. This is contained entirely within the backend
implementation — the `Backend.List(project, ...)` signature is unchanged.

### Apple `container` caveats to surface

- **Compose service DNS is not available by default.** Reject multi-service shared-network
  projects unless explicit Apple local DNS support is configured and implemented.
- **Network aliases and `hostname:` are unsupported.** Reject them for the Apple backend.
- **Networks require newer Apple networking support.** On older macOS `network create` is
  unavailable; fail clearly instead of implying Compose networking works.
- `--network host` and `--privileged` are unsupported — map compose `network_mode: host`
  and `privileged: true` to a warning; suggest `--cap-add` for specific capabilities.
- `--restart` is unsupported — `restart:` policies map to a warning.
- No healthcheck flags — `depends_on: condition: service_healthy` cannot be delegated to
  the CLI; bianpai must poll if/when it implements health gating (Stage 2).
- Anonymous volumes are **not** auto-removed by `--rm` (unlike Docker) — document for
  `down`.

### Work items

1. `internal/backend/container.go` — `Container` backend + `BuildArgv`/`RunArgv`, mirroring
   `wslc.go`.
2. `List` with JSON decode + client-side label filter.
3. Backend selection: extend `PersistentPreRunE` in `cli.go` to accept `container`,
   construct the right backend (`--backend container`); consider defaulting to `container`
   on macOS when the binary is present.
4. `internal/backend/container_test.go` — argv assertions via the fake `Runner`, mirroring
   `wslc_test.go`.
5. Capability warnings for host networking / privileged / restart / healthcheck.
6. README: flip Apple container to `[x]`.

## Stage 2 — run-loop correctness (compose-spec core semantics)

- Foreground attach mode for `up` (multiplex service logs; the current warning goes away).
- Environment interpolation: `${VAR}`, `${VAR:-default}`, and `.env` file loading — a core
  part of the compose spec not yet implemented.
- Healthcheck waiting + conditional `depends_on`
  (`service_started` / `service_healthy` / `service_completed_successfully`), polled by
  bianpai since no backend exposes it.
- `up --build`, `--force-recreate`, and idempotent recreate (hash service config into a
  label to skip unchanged containers).

## Stage 3 — broader compose-spec coverage

- Multiple `-f` files with override merge semantics.
- `profiles` (currently only warned about).
- `extends`.
- `configs` and `secrets` (file-based).
- `deploy.resources` limits mapped to `--memory` / `--cpus` (Swarm scheduling ignored).
- `restart` policy where the backend supports it.

## Stage 4 — more backends

- Docker (`docker`) and nerdctl (`nerdctl`) backends — validates the abstraction against
  the reference implementations and unlocks Linux/CI usage.

## Sequencing rationale

Smallest, highest-value first: Stage 1 is purely additive (a second `Backend`) and
delivers macOS-native usage immediately. Stage 2 fixes semantics that every backend shares
(interpolation, attach, health), so it pays off across all backends at once. Stages 3–4
broaden spec coverage and backend count once the core loop is trustworthy.

## 0.0.1 scope (definition of done)

The first tagged release proves the multi-backend thesis end-to-end on a real
multi-service project, on both backends, with today's command set.

**In:**

- Backends: `wslc` (exists) + `container` (Apple), selectable via `--backend`.
- Commands: `up` (detached), `down`, `ps`, `logs`, `build`, `pull`, `exec`, `version`.
- Compose keys: `image`, `build` (+args/target/dockerfile), `command`, `entrypoint`,
  `environment`, `env_file`, `ports`, `volumes` (bind + named), `networks` (named only),
  `labels`, `working_dir`, `user`, `depends_on` (**start ordering only**),
  `mem_limit`, `cpus`, `stdin_open`, `tty`.
- Project/label grouping; named network + volume create/remove on `up`/`down`.
- Clear warnings for anything a backend can't honor (host networking, privileged, restart,
  healthcheck, profiles, deploy).

**Explicitly out of 0.0.1** (later stages): foreground attach, `${VAR}`/`.env`
interpolation, healthcheck-gated `depends_on`, profiles, `restart`, multi-`-f` merge,
configs/secrets, deploy scheduling.

**Platform target for the `container` backend:** Apple Silicon with Apple `container`.
Host-published services are supported. Multi-service projects that require Compose
service DNS are rejected unless explicit Apple local DNS setup is added later.

## Hard shortages: Apple `container` vs Docker/Podman

These are capability gaps in Apple `container` itself, not in bianpai. bianpai can warn
around them but cannot fill them from userspace. Ranked by impact on compose workflows.

1. **Container-to-container networking is the blocker.** `container network create` and
   inter-container communication require newer Apple networking support; the vmnet APIs
   don't exist on macOS 15. More importantly for Compose, Apple `container` does not
   provide service-name DNS by default. `container network create` can create a network,
   but names such as `db`, `api`, or `etcd2` do not resolve unless Apple local DNS has
   been configured separately. bianpai must reject DNS-dependent stacks by default instead
   of starting containers that cannot talk to each other.

2. **No healthchecks.** No `--health-cmd`/`--health-interval`, no health state at all.
   `depends_on: condition: service_healthy` cannot be delegated to the CLI; bianpai would
   have to poll a readiness probe itself (Stage 2), and there is no native health to read.

3. **No restart policies.** `--restart=always|on-failure|unless-stopped` is unsupported;
   containers do not come back after crash or host reboot without an external wrapper
   (launchd/brew services). `restart:` in compose maps to a warning only.

4. **No shared network namespace / `network_mode`.** `--network host` is unsupported, and
   there is no `network_mode: service:X` / `container:X` (each container is its own VM with
   its own IP). Sidecar patterns that share a netns (proxies, debug containers) don't work.

5. **No `--privileged`.** Use targeted `--cap-add` (e.g. `NET_ADMIN`) instead; compose
   `privileged: true` can't be honored as-is.

6. **`extra_hosts`/`--add-host` is limited.** DNS can be injected via
   `--dns`/`--dns-domain`/`--dns-search`, but arbitrary single hosts-file entries aren't
   supported.

6a. **No `--hostname` / `--network-alias` mapping.** Apple `container run` does not expose
    Docker-compatible `--hostname` or `--network-alias` flags. Compose `hostname:` and
    network aliases must be rejected for the Apple backend until a supported replacement is
    designed.

7. **Anonymous volumes are not garbage-collected.** Unlike Docker, `--rm` does **not**
   remove anonymous volumes (UUID-named); bianpai must track and delete them on `down` or
   they leak.

8. **Bind-mount small-file I/O is slow.** Each container filesystem is a real ext4 block
   device over virtio; large trees over bind mounts (e.g. `npm install` against
   node_modules) are the worst case. Named volumes are preferable for hot paths.

9. **Platform floor.** Apple Silicon only (no Intel Macs); Linux containers only, each in
   its own lightweight VM (default ~1 GB RAM / 4 CPUs per container) — heavier per-service
   footprint than Docker/Podman namespaces on Linux.

10. **`container list` has no `--filter`.** Minor: bianpai filters by the project label
    client-side (already accounted for in the Stage 1 design).

Not gaps (parity exists): build with `--secret`, `container cp`, image
save/load/tag/push, `--env-file`, `--label`, per-container `--dns`.

## References

- Compose spec: <https://github.com/compose-spec/compose-spec/blob/main/spec.md>
- Apple container CLI reference:
  <https://github.com/apple/container/blob/main/docs/command-reference.md>
- Apple container how-to: <https://github.com/apple/container/blob/main/docs/how-to.md>
</content>
</invoke>
