# bianpai

![logo](https://bkimg.cdn.bcebos.com/pic/29381f30e924b899f3a1fb1765061d950a7bf604)

Bianpai (编排, biānpái, ㄅㄧㄢ ㄆㄞˊ, meaning "compose" in Chinese) is a Docker Compose-style tool designed to support multiple container backends.

## Backends

- [x] WSLC
- [x] Apple container for macOS
- [ ] Docker
- [ ] nerdctl

Select a backend with `--backend` (default `wslc`):

```sh
bianpai --backend container up
```

The Apple `container` backend requires **macOS 26 (Tahoe) on Apple Silicon** for
multi-service projects — container-to-container networking is unavailable on macOS 15, and
bianpai warns when it detects this. Some Compose keys have no equivalent yet and are
reported as warnings rather than applied: `restart`, `privileged`, `network_mode`,
`extra_hosts`, and healthcheck-gated `depends_on`. See [docs/ROADMAP.md](docs/ROADMAP.md)
for the full limitation analysis and the staged plan.

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
