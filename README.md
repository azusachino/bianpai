# bianpai

![logo](https://bkimg.cdn.bcebos.com/pic/29381f30e924b899f3a1fb1765061d950a7bf604)

Bianpai (编排, biānpái, ㄅㄧㄢ ㄆㄞˊ, meaning "compose" in Chinese) is a Docker Compose-style tool designed to support multiple container backends.

## Backends

- [x] WSLC
- [x] Apple container for macOS
- [ ] Docker
- [ ] nerdctl

Select a backend with `--backend`. The default is platform-aware: `wslc` on Windows, `container` on macOS, and no default on other platforms (specify `--backend` explicitly):

```sh
bianpai --backend container up
```

The Apple `container` backend supports host-published services and basic lifecycle operations, but it does **not** support Compose service-name DNS by default. Multi-service projects that require names like `db`, `api`, or `etcd2` to resolve are rejected with a clear limitation message until explicit Apple local DNS support is added. Some Compose keys have no equivalent yet and are reported as warnings rather than applied: `restart`, `privileged`, `network_mode`, `extra_hosts`, and healthcheck-gated `depends_on`. See [docs/ROADMAP.md](docs/ROADMAP.md) for the full limitation analysis and the staged plan.

### Apple `container` Isolation & Sharing Details

Because the Apple `container` backend runs **each container inside a separate, dedicated lightweight Linux VM** (instead of standard Linux namespaces sharing a single VM), the sharing behavior between containers is governed by hypervisor boundaries:

* **Sharable Resources**:
  * **Volumes / Bind Mounts**: Fully shareable. Multiple containers can mount the same named volume or host directory simultaneously. File modifications made by one container are instantly visible to others.
  * **Bridge Networks (IP-only)**: If containers are attached to the same custom bridge network, they reside on the same private subnet and can communicate directly using their raw IP addresses.
* **Non-Sharable / Isolated Resources**:
  * **Namespaces (Network, PID, IPC, UTS, Mount)**: Completely isolated at the hardware level. Sidecar patterns (like `network_mode: container:X` or `pid: host`) are unsupported.
  * **Service DNS**: There is no automatic service-name resolution (`http://api:8000`) between container VMs by default.
  * **VM-to-Host Hairpinning**: Connections from a VM back to the host gateway to reach a host-published port of another VM (e.g., `192.168.64.1:8082`) are blocked and dropped by the macOS `vmnet` network driver. Inter-container communication must be done using private IPs on a shared bridge network.

## Features

- [x] Compose file discovery and loading
- [x] Project, container, network, and volume naming
- [x] Basic service startup and teardown with `up` and `down`
- [x] `ps`, `logs`, `build`, `pull`, and `exec` commands
- [x] Basic image, build, command, environment, port, volume, network, label, and resource option mapping
- [ ] Full Docker Compose specification compatibility
- [ ] Foreground attach mode for `up`
- [ ] Healthcheck waiting and conditional `depends_on`
- [ ] Profiles, deploy, secrets, configs, and watch support

## Development

```sh
make check        # build + test + vet + gofmt (pre-commit gate)
make validate     # check + go mod verify (pre-PR gate)
make quality      # race detector + shellcheck
make coverage     # report Go statement coverage by package
make test-loop COUNT=20   # rerun the suite 20x to shake out flakes
make smoke        # drive up/down/ps over every usecase with a fake backend
make smoke-real   # run Apple container real checks (host-published pass + DNS-limit rejection)
```

`make smoke` (`scripts/smoke.sh`) is an end-to-end functionality check: it builds
bianpai, drives supported Apple-container stacks through `up`/`down`/`ps` with a stub
`container` CLI, and asserts DNS-dependent stacks fail with the expected limitation
message. `make smoke-real` uses the actual Apple `container` CLI: host-published stacks
must serve traffic, while stacks that require Compose service DNS must fail clearly. The
unit-level release gate lives in `TestUsecaseStacksLoad`, which asserts every usecase
parses and loads.
