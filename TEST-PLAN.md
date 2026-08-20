# CLI unit & integration test plan

**Status:** T0–T6 implemented (nightly smoke optional).

**Goal:** Automated tests for `nvm` commands so manual CLI regression is not required for every change. Complements package-level tests (`common/verify`, `installer`, `bootstrap`) already in place.

**Scope:** `certified/cli/src` (community `nvm/cli` via submodule sync).

**Out of scope (separate tracks):** `sync.exe` QuickJS worker tests, full MSI/installer E2E, real network installs in default CI, shim Zig tests (separate `shim/` plan).

---

## Current state

| Area | Coverage today |
|------|----------------|
| `commands/cfg/*` | **Strong** — set/get/del/list validation, registry persistence, stdout/JSON |
| `bootstrap/*` | **Good** — profile init, paths, shim/link layout |
| `installer/*` | **Partial** — expand, registry, reshim hook, verification |
| `commands/use` | **Minimal** — helper only |
| `commands/env` | **Helpers only** — not command `Run()` |
| Most other commands | **None** |
| Full CLI via Kong | **None** — `main.go` not testable as one entry |

**Existing patterns to reuse:**

- Isolated HKCU registry: `prefs.ROOT = "HKCU/Software/NVMTest/..."` + `TestMain` cleanup (`config_test.go`, `init_test.go`)
- Direct `Run()` on command structs (no subprocess)
- stdout capture via `os.Pipe` (`config_test.go`)
- Installer hooks via env: `NVM_RESHIM_TEST_*` (`reshim_test.go`)

---

## Test pyramid

```
                    ┌─────────────────┐
                    │  Smoke (few)    │  built nvm.exe subprocess, optional CI nightly
                    └────────┬────────┘
              ┌──────────────┴──────────────┐
              │   Integration (moderate)     │  Kong parse → Run, sandbox FS + registry
              └──────────────┬──────────────┘
        ┌────────────────────┴────────────────────┐
        │         Command unit (many)              │  Run() with harness, mocked deps
        └──────────────────────────────────────────┘
```

1. **Command unit** — call `(&commands.X{...}).Run()` with test harness (primary work).
2. **Integration** — `harness.Execute([]string{"use", "22.0.0"})` through Kong like `main.go`.
3. **Smoke** — `exec.Command("nvm.exe", ...)` against temp profile (optional, slow).

Default CI runs layers 1–2 only.

---

## Shared test harness (implement first)

**Package:** `nvm/internal/clitest` (or `cli/src/test/clitest`)

### Responsibilities

| Helper | Purpose |
|--------|---------|
| `NewSandbox(t)` | Temp dir + isolated `HKCU\Software\NVMTest\<testname>` |
| `Sandbox.Apply()` | Set `prefs.ROOT` / `prefs.ROOTS`, seed `settings.Put("root", ...)` |
| `Sandbox.Cleanup()` | `reg delete HKCU\Software\NVMTest\...`, remove temp tree |
| `CaptureOutput(fn)` | stdout/stderr capture (reuse cfg pattern) |
| `Execute(args ...string)` | Kong parse `commands.Root` + `Run`, return exit code + output |
| `SeedVersion(v, opts)` | Create `installs/vX/node.exe` (+ optional npm layout) without download |
| `ReadSetting` / `WriteSetting` | Thin wrappers over `settings.Get/Put` |

### Sandbox layout (default)

```
{TempRoot}/
  installs/          ← settings root
    v22.0.0/
      node.exe       ← stub or copy from NVM_TEST_SIGNED_NODE
  .shim/
  .cache/
  .link/
  proxy.exe          ← optional stub for reshim tests
```

### Dependency hooks (env-based, extend existing)

| Env var | Effect |
|---------|--------|
| `NVM_TEST_SANDBOX=1` | Skip `system.IsProcessStartedByExplorer` gate when harness calls bootstrap |
| `NVM_RESHIM_TEST_*` | Already stubs reshim → sync (keep) |
| `NVM_TEST_SKIP_LICENSE=1` | Skip `license.Activate()` in harness entry |
| `NVM_TEST_HTTP_FIXTURES=path` | Future: offline index/SHASUM for install/list-remote |

Harness **must not** write HKLM (community + CI safe).

### Kong entry (mirror `main.go` minus side effects)

```go
func Execute(t *testing.T, sandbox *Sandbox, args []string) (stdout, stderr string, err error)
```

- `settings.Load()` after sandbox registry applied
- `bootstrap.EnsureUserProfileInitialized()` once per sandbox when command needs profile
- Parse `&commands.Root` with same `kong.Vars` as production (`app`, `node`, `cfg_opts`)
- No Explorer pause, no event-log registration paths

---

## Command coverage matrix

Priority = implement order. **P0** first wave.

| Command | Subcommands / notes | Test type | Priority | Depends on |
|---------|---------------------|-----------|----------|------------|
| `config` / `cfg` | set, get, del, list | unit | done | — |
| `version` / `-v` | top-level flag | integration | P0 | harness |
| `default` / `current` | | unit | P0 | seed active_version |
| `env` | | unit | P0 | seed installs + settings |
| `list` / `ls` | installed (default) | unit | P0 | SeedVersion |
| `list` | cached | unit | P1 | cache fixtures |
| `list` | releases | unit | P1 | HTTP fixture / mock |
| `use` | `<version>` | unit | P0 | SeedVersion, stub reshim |
| `use` | last, lts, latest | unit | P1 | aliases + seed |
| `alias` | add, list, rm | unit | P1 | settings |
| `on` / `off` | | unit | P1 | PATH not modified; check `enabled` setting |
| `install` | validation errors | unit | P0 | no network |
| `install` | local-only / fixture .7z | integration | P2 | fixture archive |
| `uninstall` | | unit | P1 | SeedVersion |
| `cache` | view, remove | unit | P1 | seed `.cache` files |
| `rtconfig` | | unit | P2 | temp project dir |
| `reshim` | | unit | P1 | existing reshim test env |
| `doctor` / `debug` | | integration | P3 | stub sync.exe |
| `upgrade` / `subscribe` | | P3 | stub sync |
| `license` | set/clear | P3 | admin gate; skip or mock |
| `install native-tools` | | P3 | skip default CI |

### Per-command minimum assertions (P0)

**`nvm default` / `nvm current`**

- Prints active version from registry
- Empty when unset; no panic

**`nvm env`**

- JSON/text includes install root, mode, active version
- Stable keys (snapshot-style substring checks)

**`nvm list` / `nvm ls`**

- Empty install root → "No versions installed."
- Seeded `v22.0.0` → listed
- `--json` valid JSON array
- Major filter `-m 22` works

**`nvm use 22.0.0`**

- Fails if version not installed (clear error)
- Sets `active_version` when seeded
- Does not require real reshim when `NVM_RESHIM_TEST_*` set

**`nvm install` (no network)**

- Invalid semver → error before download
- `--insecure` without policy → error (existing installer gate)
- Local-only with missing archive → error (not hang)

**Kong routing**

- Unknown command → non-zero, usage hint
- Typos aliases (`nvm instal`) resolve where defined
- `nvm config set mode=invalid` → validation error (already covered)

---

## Implementation phases

Check off in order. One PR per phase where possible.

### Phase T0 — Harness foundation

- [x] Add `nvm/internal/clitest` package
- [x] `Sandbox` with registry isolation + temp install root
- [x] `CaptureOutput`, `Execute` (Kong + Root)
- [x] `SeedVersion` helper
- [x] Document env vars in package doc
- [x] One smoke test: `Execute("default")` with empty profile

### Phase T1 — Read-only commands (P0)

- [x] `default` / `current` tests
- [x] `env` tests (text + JSON if supported)
- [x] `list` installed tests (empty, seeded, JSON, major filter)
- [x] Kong routing tests (unknown cmd, help does not panic)

### Phase T2 — Mutating local commands (P0–P1)

- [x] `use <version>` success + failure paths
- [x] `use last` with `previous_active_version`
- [x] `alias add/list/remove`
- [x] `on` / `off` toggle `enabled` (no real PATH surgery — assert settings only, or mock `mode` package if needed)

### Phase T3 — Install/uninstall without network (P1–P2)

- [x] `uninstall` removes seeded version dir + registry
- [x] `install` validation-only cases
- [ ] Optional: fixture `.7z` + local mirror path for one happy-path install integration test
- [ ] Wire `verify` tests into install fixture path (signed node in archive — future)

### Phase T4 — Cache & list-remote (P1)

- [x] `cache list` / `cache remove` with seeded files
- [x] `list releases` with HTTP test server serving static `index.tab` fixture

### Phase T5 — Sync-adjacent & smoke (P3)

- [x] `reshim` via command `Run()` + existing test hook
- [x] `doctor` / `upgrade` with fake `sync.exe` script (exit 0, no QuickJS)
- [ ] Optional nightly: build `nvm.exe`, run scripted smoke suite

### Phase T6 — CI & docs

- [x] `go test ./...` from `certified/cli/src` in CI (Windows runner)
- [ ] `go test ./...` for `common/*` modules in workspace
- [x] Note in `docs/guide/builds/builds.md`: running CLI tests locally

---

## Fixtures (add under `cli/src/testdata/`)

| Path | Use |
|------|-----|
| `testdata/releases-index.json` | `list releases` offline |
| `testdata/shasums.txt` | install verify + local SHASUM airgap tests (Track 4) |
| `testdata/node-minimal.7z` | optional local install (small, license-safe) |
| `testdata/project/.nvmrc` | `rtconfig` command |

Keep large binaries out of git until needed; document generation script if `.7z` added.

---

## What we deliberately do not test in unit layer

- Real download from nodejs.org (flaky, slow)
- Authenticode on every CI machine for install E2E (use `NVM_TEST_SIGNED_NODE` locally; skip in CI if absent — same as `common/verify`)
- HKLM / elevated license paths (certified-only; separate enterprise test job)
- Shim/proxy Zig binaries spawning real node (runtime verify phase — separate)
- Full MSI install/uninstall

---

## Success criteria

- `go test ./...` in `cli/src` runs in **&lt; 30s** on dev machine (no network)
- Every **P0 command** has at least: success path, one failure path, output assertion
- Regressions in settings/registry routing caught without manual `nvm` typing
- New commands add row to coverage matrix + tests before merge (convention)

---

## Relationship to security work

| Security track | CLI test support |
|----------------|------------------|
| Track 1a verify | Done (`common/verify`, installer wrapper) |
| Track 4 download cache | Done (`cache_integrity_*_test.go`, `verifycache/archive_cache_test.go`) — supersedes old “Track 1b cache SHASUM” label |
| Phase 0 shim ACL | Done (`common/fs/shim_lock_*`, bootstrap/reshim tests) |
| Runtime verify cache | Done (Phases 0–7); shim tests separate; CLI asserts install/use/reshim wiring |
| Track 5 sync trust | `sync/syncsafe/*_test.go`, `sync/commands/update_manifest_test.go` |

---

## Open decisions (resolve in T0)

- [ ] Package name: `internal/clitest` vs `test/clitest` (prefer **`nvm/internal/clitest`** — importable only from cli module)
- [ ] Single global `TestMain` for all command tests vs per-package (prefer **shared `clitest` + per-package tests**, no global TestMain outside harness)
- [ ] Golden files for `env`/`list` output vs substring asserts (start with **substring**; goldens if output churns)
- [ ] Refactor `main.go` to call `cli.Run(args)` for testability (optional T0 — reduces duplication)

---

## Suggested start after approval

**Phase T0** — `clitest` sandbox + `Execute` + one `default` test.

Then **T1** read-only commands (fast value, no network, no install).
