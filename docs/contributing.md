# Contributing

Contributions are welcome: bug reports, documentation fixes, and code.

## Development setup

You need [Go 1.25+](https://go.dev/dl/). For the documentation site you also need Node.js 18+.

```sh
git clone https://github.com/omar0ali/watchd.git
cd watchd

scripts/run.sh test        # go vet + go test -race ./...
scripts/run.sh dev ps      # run the CLI from source
scripts/run.sh build       # ./build/watchd with the version stamped in
```

To try changes without touching your real daemon, point watchd at separate directories:

```sh
export WATCHD_RUNTIME_DIR=/tmp/watchd-dev XDG_STATE_HOME=/tmp/watchd-dev-state
```

## Architecture

```text
 watchd CLI ──(JSON over unix socket)──▶ watchd daemon
 (stateless)                             ├── project "shop"  (Lua state for hooks)
                                         │    ├── app api     process group, log buffer, health checks
                                         │    ├── app web     waits for api
                                         │    └── watchers    fsnotify + debounce, one per app
                                         └── project "blog"
```

| Package | Responsibility |
| --- | --- |
| `cmd/` | Cobra CLI: turns commands into requests and formats the output. `ui.go` is the Bubble Tea dashboard, and `init.go` holds the Huh prompts. |
| `internal/config` | Lua sandbox, strict parsing and validation, dotenv files, hooks. |
| `internal/detect` | Recognises common project types and renders a starter `watchd.lua`. |
| `internal/supervisor` | Per-app supervision loop, process groups, restart policies, health checks, log buffers. |
| `internal/watcher` | Recursive fsnotify watching, glob matching, debouncing. |
| `internal/daemon` | Project registry, request routing, lifecycle, signals, lock. |
| `internal/ipc` | Socket server and client, peer credential checks. |
| `internal/paths` | Runtime and state directories and their permission checks. |
| `internal/api` | Request, response and status types shared by the CLI and the daemon. |

## Commit messages

The version number of each release is worked out from commit messages, so please follow
[Conventional Commits](https://www.conventionalcommits.org):

| Commit message | Example | Release |
| --- | --- | --- |
| `feat!:`, `fix!:` or a `BREAKING CHANGE:` footer | `feat!: rename restart policies` | major (minor while below v1.0.0) |
| `feat:` | `feat(logs): add --since flag` | minor |
| `fix:`, `perf:`, `refactor:` or no prefix | `fix: crash on empty env_file` | patch |
| only `docs:`, `ci:`, `chore:`, `test:`, `style:`, `build:` | `docs: add recipes` | no release |

## Releases

Releases are automated. Every push to `main` runs the tests on Linux and macOS. If they pass,
the release workflow computes the next version from the commits since the last tag, pushes
the tag, and publishes a GitHub release with binaries for Linux and macOS (amd64 and arm64),
checksums and release notes grouped by type.

- Add `[skip release]` to a commit message to push without releasing.
- To release by hand, for example to force a version bump: **Actions → Release → Run
  workflow**.
- Preview the next version locally with `scripts/next-version.sh`, and test the build
  without publishing with `goreleaser release --snapshot --clean`.

## Documentation

This site is built with [VitePress](https://vitepress.dev) from the Markdown files in
[`docs/`](https://github.com/omar0ali/watchd/tree/main/docs). It's published to GitHub
Pages automatically whenever `docs/` changes on `main`.

```sh
cd docs
npm install
npm run dev          # live preview at http://localhost:5173/watchd/
```

Every page has an **Edit this page on GitHub** link at the bottom.
