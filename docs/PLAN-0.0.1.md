# Detailed Plan — Stage 1 / 0.0.1: Apple `container` backend

Tracked in asobi as epic `bianpai:apple-container` (tasks `task-1..task-5`).
See [ROADMAP.md](./ROADMAP.md) for scope and the hard-limitation analysis.

The `container` backend mirrors `internal/backend/wslc.go`: pure argv-rendering methods
plus thin `Runner` wrappers, tested by asserting argv against a fake `Runner`. The compose
and cli layers are largely untouched; the only cli change is backend selection.

---

## task-1 — `Container` backend: argv rendering + tests

**Files:** new `internal/backend/container.go`, new `internal/backend/container_test.go`.

Add a `Container` type implementing `backend.Backend`, structured exactly like `WSLC`
(`runner Runner`, `binary string = "container"`, `NewContainer(runner Runner) *Container`).
Everything is a straight argv render except `List` (deferred to task-2 — stub it to call
`container list --all --format json` for now, filtering added there).

**Command differences from WSLC** (the only places the two backends diverge):

| Method          | WSLC                         | Container                                  |
| --------------- | ---------------------------- | ------------------------------------------ |
| `Check`         | `wslc --version`             | `container --version`                      |
| `Pull`          | `wslc pull <img>`            | `container image pull <img>`               |
| `Remove`        | `wslc remove <name>`         | `container delete <name>`                  |
| `CreateNetwork` | `wslc network create`        | `container network create --label .. <n>`  |
| `RemoveNetwork` | `wslc network remove <n>`    | `container network delete <n>`             |
| `CreateVolume`  | `wslc volume create`         | `container volume create --label .. <n>`   |
| `RemoveVolume`  | `wslc volume remove <n>`     | `container volume delete <n>`              |

`Build`/`Run`/`Stop`/`Exec`/`Logs` render identically to WSLC (same flag names:
`--tag`/`--file`/`--build-arg`/`--target`/`--no-cache`/`--pull`, and
`--detach`/`--rm`/`--name`/`--env`/`--env-file`/`--label`/`--publish`/`--volume`/`--network`/
`--workdir`/`--user`/`--memory`/`--cpus`/`--entrypoint`/`--interactive`/`--tty`).
Do not copy unsupported Docker-compatible flags such as `--network-alias` or
`--hostname`; the Apple backend must reject Compose features that require them.

**Tests** (`container_test.go`): clone `TestWSLCRunArgv` / `TestWSLCBuildArgv` as
`TestContainerRunArgv` / `TestContainerBuildArgv` with `container` as argv[0]. Add
`TestContainerCommandArgv` asserting the diverging commands: `Pull` →
`[container image pull nginx]`, `Remove` → `[container delete name]`, network/volume
create/delete argv.

**Done when:** `go test ./internal/backend/` passes; `Container` satisfies `Backend`
(add a `var _ Backend = (*Container)(nil)` assertion).

---

## task-2 — `List` with client-side project-label filter

**Files:** `internal/backend/container.go` (+ test).
**Depends on:** task-1.

`container list` has **no `--filter`** (unlike WSLC). Implement `List` by capturing JSON
and filtering in Go:

1. Run `container list --all --format json`, capturing stdout into a `bytes.Buffer`
   (pass the buffer as the runner's stdout, not the caller's).
2. Decode into `[]struct{ ... }` — decode leniently; the fields we need are the
   container name/id, status, and **labels**. Inspect real output shape with
   `container list --all --format json` and pin the struct to it (labels may be a nested
   `configuration.labels` map or a flat field — verify before coding).
3. Keep rows whose labels contain `com.bianpai.project=<project>`.
4. Render a compact table (NAME / IMAGE / STATUS) to the caller's stdout. Reuse or add a
   small tab-writer helper; match WSLC's `ps` output loosely (exact columns can differ).

**Risk / verify:** the JSON schema is the one unknown. Before implementing, run the real
CLI (or `container inspect`) to confirm the label path. If labels aren't in `list` output,
fall back to `container list --all --format json -q` for ids + `container inspect <id>`
per container to read labels. Note this fallback in the code comment.

**Tests:** feed a canned JSON blob through a fake `Runner` that writes it to stdout, assert
only project-matching rows are rendered. This task is correctness-sensitive (parsing +
filtering) — do not dispatch to the cheapest model without a review gate.

---

## task-3 — Backend selection in the CLI

**Files:** `internal/cli/cli.go`.
**Depends on:** task-1.

Today `Run` hardcodes `be: backend.NewWSLC(nil)` and `PersistentPreRunE` only validates the
name. Change:

1. Drop the eager `be:` assignment in `Run`; construct the backend in `PersistentPreRunE`
   (flags are parsed by then) via a small factory:
   ```go
   func newBackend(name string) (backend.Backend, error) {
       switch name {
       case "", "wslc":     return backend.NewWSLC(nil), nil
       case "container":    return backend.NewContainer(nil), nil
       default:             return nil, fmt.Errorf("unsupported backend %q", name)
       }
   }
   ```
   Assign `a.be` there. Keep the default flag value `wslc` (explicit opt-in to
   `--backend container`) for 0.0.1 — auto-detection on macOS is a later nicety.
2. Guard: `version`/help must still work without a project; backend construction has no
   side effects so this is fine.

**Tests:** a cli-level test that `--backend container up` selects the container backend —
inject a fake `Runner` via a seam, or assert `newBackend("container")` returns
`*backend.Container`. Keep it light; argv coverage lives in task-1/2.

---

## task-4 — Capability warnings + macOS-26 gating

**Files:** `internal/compose/load.go` (extend `unsupportedWarnings`),
`internal/backend/container.go` or `internal/cli/cli.go` (platform check).
**Depends on:** task-3.

Surface the hard limitations as warnings rather than silent misbehavior:

1. Parse (or peek at) the compose keys that the container backend cannot honor and warn
   once each: `network_mode: host`, `privileged: true`, `restart:`, `extra_hosts`. Extend
   the existing `unsupportedWarnings` pattern in `load.go` (add minimal `yaml.Node` fields
   to `Service` to detect presence — mirror how `deploy`/`healthcheck` are already peeked).
2. **macOS-26 gate:** when backend is `container` **and** the project has >1 service (or any
   named network beyond `default`), check the host: run `container network ls` and, if it
   errors / is unavailable, emit:
   `Apple container backend does not support Compose service DNS by default; configure Apple container DNS support or use host-published addresses`.
   Prefer probing the CLI over parsing `sw_vers` so it stays a capability check, not a
   version guess. Do this in the container backend's `Check`, or once in cli before `up`.

**Tests:** `load_test.go` case asserting the new warnings fire for a service using
`restart`/`privileged`. The platform probe is environment-dependent — keep it out of unit
tests or gate behind an injected probe func.

---

## task-5 — Docs + README

**Files:** `README.md`, minor `docs/ROADMAP.md` touch-up.
**Depends on:** task-4.

- Flip `- [ ] Apple container for macOS` → `- [x]` in README Backends.
- Add a short "Backends" usage note: `bianpai --backend container up`, and the Apple
  container service-DNS limitation (link the ROADMAP limitation section).
- Note the known caveats users will hit: no restart policy, no healthcheck gating yet,
  anonymous volumes not auto-removed.

**Done when:** README reflects reality and a fresh reader knows the platform floor.

---

## Sequencing & dispatch

```
task-1 (argv + tests)          READY
  ├─ task-2 (List filter)      BLOCKED_ON task-1   [correctness-sensitive]
  └─ task-3 (backend select)   BLOCKED_ON task-1
        └─ task-4 (warnings+gate)  BLOCKED_ON task-3
              └─ task-5 (docs)     BLOCKED_ON task-4
```

Sequential dispatch (default): one task at a time, review the diff, `make check`, commit
per task. task-2 is the only one carrying real logic risk (JSON shape) — verify against the
real `container` CLI before/while implementing, and gate it on review.
</content>
