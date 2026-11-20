# Portico

Dokku-like PaaS in Go: a single CLI (`portico`) that runs on a Linux server and
deploys apps as Docker Compose projects behind a Caddy reverse proxy.

## Commands

```bash
make build        # go build with -X main.version → bin/portico
make test         # go test -v ./...
make lint         # golangci-lint run (v2 config; CI pins v2.5.0)
make vet / fmt
go mod tidy       # CI fails if go.mod/go.sum are not tidy
bash -n install.sh
```

Module path is `github.com/maxvegac/portico` with `replace => ./src`; all code lives under `src/`.
CI (`.github/workflows/build.yml`) uses Go 1.25; `go.mod` and the Dockerfile stay on Go 1.21,
so don't add dependencies that require a newer `go` directive.

## Layout

- `src/cmd/portico/main.go` – root command; `var version` is injected by ldflags into `commands.Version`.
- `src/cmd/portico/commands/` – one file per command.
  - `app_helpers.go`: `applyApp` (lock → save → `docker compose up` → `syncProxy`). Every command that mutates an app must go through it.
  - `nested.go`: dispatch for groups whose subcommand comes after positionals (`portico env <app> [service] add K V`). Those groups use `DisableFlagParsing` + `newNestedRun`.
  - `addons.go` / `runAddonsNested`: `portico addons <name> <action>`.
  - `addon_tools.go`: admin tool addons (adminer, mongo-express).
  - `ssh_command.go`: forced command for restricted SSH keys (git push only).
  - `git_receive.go`: post-receive hook; only `main`/`master` are deployed.
- `src/internal/app` – App model; loads/saves from `docker-compose.yml`.
- `src/internal/docker` – compose generation (`GenerateDockerCompose`) and deploy.
- `src/internal/addon` – addon definitions and `addons/config.yml` instances.
- `src/internal/proxy` – writes the main Caddyfile and reloads Caddy.
- `src/internal/embed` – embedded templates (`templates/*.tmpl`) and static files (reverse proxy, addon definitions `static/addons/definitions/*.yml`).
- `src/internal/util` – validation (`ValidateAppName`, `ValidateNewAppName`, …), atomic writes, flock lock, ownership fixes.

## Key design rules

- **`docker-compose.yml` is the source of truth** for an app. Portico metadata lives in `x-portico` (domain, http_port, http_enabled, http_service, replicas, disabled_http_port, generated_hash). Unknown keys and manual edits must survive regeneration.
- Server layout: `/home/portico/{apps,repos,addons/{definitions,instances},reverse-proxy,templates,logs,www}`. Config is `/home/portico/config.yml` (or `/etc/portico`, or `$PORTICO_CONFIG`). It is never read from the current directory.
- Networking: everything joins the external `portico-network`. App services get the alias `<app>-<service>`; addon instances use the compose project and alias `addon-<instance>` (`util.AddonPrefix`). App names `reverse-proxy`, `portico`, `addon` and `addon-*` are reserved to avoid compose project collisions (deploys use `--remove-orphans`).
- Caddy: the main Caddyfile imports `/home/portico/apps/*/Caddyfile` and `/etc/caddy/addons/*.Caddyfile` (`reverse-proxy/addons/`, for published admin tools). Caddyfiles use tab indentation. Bind-mounted files are written in place (not rename), and the app Caddyfile carries a hash comment to detect manual edits.
- Addon DB ports are bound to `127.0.0.1` only; passwords are random (`util.RandomSecret`). DB names and users interpolated into SQL must pass `util.ValidateDBIdentifier`.
- Commands may run as root (sudo) or as the `portico` user. After writing files as root, call `util.FixFileOwnership` / `FixDirOwnership`.
- Use `util.WriteFileAtomic` for config/state files.

## Testing tips

- Point at a sandbox home with `PORTICO_CONFIG=/path/to/config.yml` (set `portico_home`, `apps_dir`, `proxy_dir`, `addons_dir`, `templates_dir`, `external_ip`).
- To exercise the full flows without pulling images, put a fake `docker` script first on `PATH` that logs its args and exits 0 (delegate `compose config` to the real binary).
- Validate generated files with `docker compose config -q` and `caddy validate --adapter caddyfile`.

## Conventions

- User-facing CLI output is in English; keep it consistent.
- Commands print errors with `fmt.Printf("Error: ...")` and return. They don't call `os.Exit`, except to propagate the exit code of exec/shell.
