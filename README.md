# bianpai

Bianpai (编排, biānpái, ㄅㄧㄢ ㄆㄞˊ, meaning "compose" in Chinese) is a Docker Compose-style tool designed to support multiple container backends.

## Backends

- [x] WSLC
- [ ] Apple container for macOS
- [ ] Docker
- [ ] nerdctl

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
