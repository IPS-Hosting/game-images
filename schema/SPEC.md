# Game Manifest v2 — Specification

Normative specification of the IPS Hosting game manifest (`game.yaml`, `apiVersion: 2`).

Machine-readable schema: `schema/game-manifest.v2.json` (JSON Schema draft 2020-12).
Linter: `schema/lint.mjs` (JSON Schema + the semantic checks in §9). In-repo conformance
manifests: `valheim/`, `minecraft/`, `palworld/` — together they represent the three
onboarding archetypes:

| Example | Exercises |
|---|---|
| `valheim.yaml` | plain steam game: argv-only start, a2s query, docker stop, grouped ports, file permissions |
| `minecraft.yaml` | external image (itzg) with dynamic tag via go-template, `install: none`, computed env (JVM heap), minecraft query fallback, `installable: false` |
| `palworld.yaml` | conditional argv (`when`), server context (public IP/port), RCON query + RCON stop, HIDDENSTRING password rendered into config via the template escape hatch |

## 1. Purpose & scope

A manifest is the **complete, product-independent definition of a game**: identity, catalog
metadata, runtime image, install procedure, command line, config files, ports, settings, file
permissions, query/stop behavior, panel flags and minimum requirements. It lives at
`<game>/game.yaml` in the `IPS-Hosting/game-images` repo (one directory per game; the directory
name **must** equal `short`). It is consumed by:

1. **game-images CI** — JSON-Schema lint + semantic checks (§7), optional boot harness.
2. **api sync** — upserts the DB game data (§6). Merge to main = live within minutes.
3. **ipsd** — the single generic game engine renders everything per game server (§4–§5).

There are **no pricing fields** — the manifest must not encode anything commercial. Price
display derives from `requirements` joined with platform tariffs. The platform's 17 existing
games are migrated to manifests later (out of scope here).

## 2. Identity & catalog

| Field | Type | Req | Notes |
|---|---|---|---|
| `apiVersion` | int | ✔ | MUST be `2`. Importers reject other values. |
| `short` | string | ✔ | `^[a-z0-9]{1,32}$`, unique platform-wide, = directory name. The universal key (DB, GraphQL, daemon). |
| `name` | string | ✔ | German display name, ≤64 chars. |
| `description` | string | ✔ | German markdown for the landing page (rendered server-side, no raw HTML). |
| `image` | string\|null | – | Logo/cover URL (Storyblok). Nullable. |
| `tags` | string[] | – | 0–6 tags from the controlled vocabulary (schema `$defs.tag`, Steam-inspired, ~24 values). Name search is the primary discovery mechanism; the list is extended deliberately, never per-game. |
| `consoleCommandsEnabled` | bool | – | Gates console command input and `SEND_COMMAND` cron jobs (existing `Game` column). |

## 3. Runtime & install

### 3.1 `runtime`

- `image` (required): container image reference. **May be a go-template** (§4). The rendered
  value MUST match `^[a-z0-9][a-z0-9./:_-]*(:[a-zA-Z0-9._-]+)?$`. External registries are
  first-class (`itzg/minecraft-server:...`).
- `dataPath` (default `/home/ips-hosting`): mount point of the persistent data volume.
  MUST be an absolute path. Our images ship `VOLUME /home/ips-hosting`; external images declare
  their own (`itzg`: `/data`). The container always runs as the platform's fixed non-root user
  (uid/gid 4711) with hardened defaults (read-only rootfs, caps dropped, no-new-privileges) —
  external images MUST tolerate this.

Game images are **dumb**: base + apt dependencies only, no entrypoint logic. A small curated
family covers most games (`game-steam`, `game-java:<ver>`, `game-wine`, `game-dotnet`, …);
per-game images and external images are escape hatches.

### 3.2 `install`

`kind: steam` — sidecar containers (same image family) run steamcmd, args built by ipsd:
`+force_install_dir <dataPath> +login anonymous|<host-credentials> +app_update <appId>
[validate] +quit` (+ `+@sSteamCmdForcePlatformType` when `forcePlatform` is set, + beta-branch
args when `betaBranchSetting` yields a non-empty branch; `betaPasswordSetting` is then included).
`login: anonymous` (default) or `login: host` (shared host credentials from ipsd config, for
games that cannot use anonymous). The installer runs in `ipshosting/steamcmd` (overridable via
`install.image`). `extraArgs` are literal-only (no templates, max 8).

`kind: none` — the image installs/updates itself (itzg pattern). ipsd never runs install
sidecars; `flags.installable` MUST be `false`.

`kind: archive` — download `url`, verify `checksum` (sha256), extract (`format`,
`stripComponents`) into `dataPath`, via a small utility image (`ipshosting/game-tools` default).

Update and validate reuse the same engine (steam: `app_update` with/without `validate`;
archive: re-download + checksum). Successful install/validate marks the server installed.

### 3.3 Reserved runtime contract (all images)

`TZ` and `HOME` are always set by ipsd and MUST NOT be set in `env`. The container command is
`args` (exec argv, no shell) — for external images that own their entrypoint, `args` is empty
and the image's entrypoint runs.

## 4. Template engine

Templates are **go text/template** with a restricted context and function whitelist. They are
used in: `runtime.image`, `args` (each item), `env` values, `config[].values` values,
`config[].template` (full file), `argItem.when`.

### 4.1 Context

| Expression | Meaning |
|---|---|
| `{{ .Setting "key" }}` | resolved setting value (currentValue → defaultValue → ""), string |
| `{{ .SettingBool "key" }}` | boolean of a BOOLEAN setting ("1"/"true" → true) |
| `{{ .SettingInt "key" }}` | integer value of a NUMBER setting |
| `{{ .SettingList "key" }}` | STRINGLIST as []string |
| `{{ (.Port "game").Container }}` / `.Exposed` / `.Protocol` | allocated port; listen args use Container, advertised/public ports use Exposed |
| `{{ .Server.ID }}`, `{{ .Server.IPv4 }}`, `{{ .Server.CpuThreads }}` (float), `{{ .Server.RamMB }}`, `{{ .Server.DiskGB }}` | game-server facts |
| `{{ .Timezone }}` | platform timezone (also set as `TZ`) |

Unknown setting/port keys referenced by a template are a **validation error** (static check in
CI/importer; ipsd fails loud at runtime).

### 4.2 Function whitelist (no sprig)

`default` · `bool` (`true`/`false`) · `boolInt` (`1`/`0`) · `ternary a b cond` ·
`eq ne lt le gt ge` · `int` (float→int) · `intf` (string/number→float) · `add sub mul div`
(int-preserving when all operands are ints, else float) · `max min` · `lower upper trim` ·
`quote` (wraps in `"…"`) · `replace old new` · `printf` (stdlib).

Real-world coverage shown in the examples: itzg tag switch (if/else), JVM heap arithmetic
(sub/ternary/mul/div/printf), conditional argv (ternary/`when`).

### 4.3 Truthiness & empty-drop

- `when`: rendered, trimmed. Falsy = empty string, `0`, `false` (case-insensitive). Else truthy.
- argv items that render (arg or value) to an empty string are **dropped** — this is how
  optional flags with always-present pairs work when combined with object form.

### 4.4 Output validation

- `runtime.image` render MUST match the image-reference pattern (§3.1) — evaluated with the
  settings' default values in CI, with live values per server in ipsd (fail loud on mismatch).
- Rendered argv items MUST NOT contain NUL bytes. No other escaping is applied: argv is passed
  through the Docker exec API, never a shell.
- Config values are rendered as raw strings; the format adapters handle syntax (quoting is the
  template's responsibility — use `quote` where the target format needs it).

## 5. Command line, env & config files

### 5.1 `args`

Ordered list. Item = template string (one argv item) or object
`{ arg, value?, when? }` (`value` = appended second argv item). `ipsd` renders each item with
the template engine, applies §4.3, and execs the result — **no shell**. Working directory is
`dataPath`; the process runs as the fixed non-root user.

### 5.2 `env`

Map `NAME` → template (`^[A-Z][A-Z0-9_]*$`, `TZ`/`HOME` reserved). Rendered per server, passed
via the Docker env API (no shell interpolation).

### 5.3 `config`

Rendered into the data volume **before every start** (and after install). Semantics:

- Structured formats (`properties`, `ini`, `json`, `yaml`, `kv`): read the existing file at
  `path` (create if missing), apply managed `values` (dot-notation keys for nested formats:
  `Section.key` for ini; nested paths for json/yaml), preserve unmanaged keys when
  `merge: true` (default), write back atomically (tmp + rename, fsync dir).
- `format: template`: render the full file (overwrite semantics) — the escape hatch for
  formats the adapters cannot express (e.g. palworld's one-line
  `OptionSettings=(...)` tuple). Directories are created as needed.
- `kv`: lines `key<delimiter>value` (default `=`); `delimiter` configurable.
- Paths are relative to `dataPath`; `path` MUST NOT escape it (ipsd enforces, same chrootfs
  rules as user file access).

Managed keys are the **single source of truth**: manual edits to managed keys are overwritten
on the next start (documented in the panel); unmanaged keys survive.

## 6. Ports, settings, file permissions

### 6.1 `ports` — maps to `GamePort` rows

`identifier` (unique per game; semantic key used by templates, e.g. `game`, `query`, `rcon`),
`protocol` (TCP/UDP), `default` (allocation start), `group` (ports in a group increment
together — keeps fixed spacing, e.g. valheim query = game+1, minecraft query = game), `static`
(container port pinned to `default`). Allocation logic is unchanged (machine capacity/IP model).

### 6.2 `settings` — maps to `GameSetting` (+ Category + Value rows)

One-to-one with the existing model: `key` (unique per game), `name`/`description` (German),
`type` (8 types), `validation` (validatorjs), `default` (string-encoded; BOOLEAN `'0'/'1'`),
`sortOrder`, `category` (shared platform set: Allgemein, Server Properties, Steam Beta Branch,
RCON, … — resolved by name), `values` (required for `*ENUM`). `HIDDENSTRING` values are stored
encrypted and are the prescribed way to pass secrets (RCON passwords, GSLT-like tokens) into
configs. The platform value stack stays: currentValue → defaultValue → template `default`
helper.

### 6.3 `filePermissions` — maps to `GameFilePermission` rows

Glob `match` (relative to `dataPath`), tri-state `allowRead`/`allowWrite` (`null` = inherit
next-lower priority), `priority` 0–255. Convention: deny writes on binaries/runners
(priority ≥ 10), leave data/config writable.

## 7. Stop & query (replaces per-game Go capability interfaces)

- `stop`: `docker` (plain docker stop; `timeoutSeconds`, default 10) · `stdin` (`command` over
  the attached stdin; wait ≤ timeout; fallback docker stop) · `rcon` (same over RCON on `port`
  with `passwordSetting`).
- `query`: `a2s` (Valve A2S on `port`) · `minecraft` (see matrix below) · `rcon` (`command` +
  `pattern` regex, optional `nameGroup`/`mapGroup`/`maxPlayersGroup`) · `http` (JSON endpoint +
  dot-paths) · `none`. Query drives player counts, the generic freeze detection (>3 consecutive
  failures → kill+restart) and server-name sync.
  **Minecraft protocol matrix** — the right protocol depends on server type and version, so the
  engine (not the manifest) owns the fallback chain, mirroring the production-tested
  `minecraft.go`:
  - *Real servers* (Vanilla, Paper, Spigot, Forge, Fabric, …): GameSpy4 UDP query
    (`enable-query`, since MC 1.0) **and** TCP server-list ping (SLP) both available.
  - *Proxies* (BungeeCord, Velocity): SLP TCP only — no GameSpy4 listener exists.
  - *Version*: SLP formats differ (beta 1.8–1.3 legacy kick-packet, 1.4–1.5, 1.6, 1.7+ modern
    handshake/status). Manifests: `protocol: minecraft` + `port` (GameSpy4 UDP, `enable-query`
    set via env) + optional `fallbackPort` (TCP SLP) = **auto** (UDP primary, TCP fallback);
    `protocol: minecraft` + `port` without `fallbackPort` = **SLP-only** (proxies). SLP-only
    games MUST NOT set `fallbackPort`; the version-specific SLP variants are handled by the
    library, not per-game manifests.
  The archived `wisp-gg/go-gamequery` dependency (MIT; Go module frozen at its 2021 release) is
  replaced by focused, maintained libraries: `rumblefrog/go-a2s` (A2S) and
  `dreamscached/MineQuery` (SLP variants beta 1.8 → 1.7+ **and** GameSpy4 query), keeping
  `gorcon/rcon` (already used); HTTP probing is native. A fork of go-gamequery is the fallback
  if a new protocol ever needs its multi-protocol surface; the `query.protocol` enum grows
  additively when that happens.

## 8. Workshop, CurseForge, flags, requirements

- `workshop`: generic steam-workshop feature. `settings` lists keys whose values hold numeric
  workshop item IDs (STRINGLIST) and/or collection IDs (STRING, only with
  `collections: true`). Engine contract: resolve + install into `<dataPath>/workshop/<fileId>`
  before start, idempotent, logged; game-specific layout adaptations are engine design work
  (v2 scope), not manifest surface.
- `curseforge`: `{ gameId }` enables CurseForge mod browsing/install for the game (api gateway
  `GAME_SHORT_TO_CURSEFORGE_GAME_ID` becomes sync-driven).
- `flags`: replaces the frontend's hardcoded short-lists — `renderLogo` (transparent logo vs
  cover), `steamConnect` (steam:// link), `showMap`, `showPlayerCount`, `installable`
  (false ⇔ `install.kind: none`).
- `requirements`: `minCpuThreads` (number, e.g. 0.5), `minRam` (MB), `minDiskspace` (GB) — the
  Gamebox gate for ordering and game switching. Optional/empty allowed (unknown games);
  initial values via manual/web research, later refined at runtime from Prometheus usage
  data (e.g. p95 per game).

## 9. Validation

Two layers, same rules:

1. **JSON Schema** (`game-manifest.v2.json`): structure, types, patterns, enums, conditional
   requirements (§3–§8). `additionalProperties: false` everywhere → unknown keys rejected.
2. **Semantic validator** (implemented in the game-images CI lint and the api importer):
   - `short` == directory name; directory name matches `^[a-z0-9]{1,32}$`.
   - every port referenced by `query.port`/`fallbackPort`, `stop.port`, and templates
     (`.Port "id"`) exists in `ports`; port identifiers unique.
   - every `.Setting "key"` referenced anywhere resolves to a declared setting; setting keys
     unique; `*ENUM` types have `values`; BOOLEAN defaults match `^[01]$`.
   - `workshop.settings` keys exist; `install.steam.betaBranchSetting`/`betaPasswordSetting` and
     `stop/query.passwordSetting` exist (and are HIDDENSTRING where a password is expected).
   - `install.kind == none` ⇔ `flags.installable == false`.
   - `query.fallbackPort` is only allowed with `protocol: minecraft` (auto mode).
   - `runtime.image` renders (with default settings) to a valid image reference.
   - reserved env (`TZ`, `HOME`) not overridden; `args`/`env`/`config` reference no undeclared
     template functions.
   - `requirements` values positive; `minRam` MB, `minDiskspace` GB.
   - no field value contains a secret-looking token (heuristic warning) — secrets live in
     HIDDENSTRING settings, never in the manifest.

## 10. Sync semantics (api)

- Trigger: GitHub webhook on push (HMAC-authenticated endpoint) + hourly cron re-sync.
- Source: `main` of game-images (protected); manifests fetched with ETag caching.
- Upsert: Game (identity/catalog/flags/requirements/runtime image contract and the raw
  manifest document), GamePort, GameSetting
  (+ category resolution by name, GameSettingValue), GameFilePermission. Manifest is the
  source of truth; the importer reports a dry-run diff first; games with existing servers are
  never deleted (removal is a deliberate ops action, not a sync side effect).
- Per-server values (`GameServerSetting.currentValue`) are untouched; only definitions and
  defaults change.
- `image: null` leaves the existing DB image untouched (catalog images are curated in
  Storyblok; the manifest only overrides when a URL is set).

## 11. CI (game-images)

- `lint` job on every PR touching `<game>/game.yaml`: JSON-Schema validate + semantic
  validator (the same binary/config as the api importer), plus cross-file checks
  (`short` = dir, tags vocabulary from the schema).
- Optional `boot` harness per game (reusable workflow_call): build image → install → start →
  wait for a per-game ready line (declared in the workflow, not the manifest) → stop → assert
  exit; RAM/CPU profiles for `requirements` verification.
- Existing per-game build workflows stay (path-filtered); image `VERSION` bumps remain manual.

## 12. Versioning & evolution

- `apiVersion: 2` is the contract. Within v2, changes are **additive only** (new optional
  fields); breaking changes require apiVersion 3 and a dual-version importer window.
- The tag vocabulary, setting categories and port identifier conventions are platform-managed
  lists (extended deliberately, reviewed in the game-images repo).

## 13. Security notes

- No shell anywhere in the runtime path (argv exec, env API, atomic file writes).
- Templates run with a whitelist context — no OS access, no arbitrary code; rendered image refs
  are pattern-validated.
- Secrets only as HIDDENSTRING settings (encrypted at rest, AES-256-CBC platform store); never
  in manifests; the CI secret-heuristic check blocks token-looking literals.
- `config[].path` and file permission globs are chroot-checked against `dataPath` (same symlink
  escape protection as user file access).
- The importer only trusts `main` of the protected repo; the webhook verifies an HMAC signature.

## 14. Deferred / future extensions

- More query protocols (only as needed; engine-side, no schema churn — enum grows).
- More install kinds (e.g. git) — additive.
- `restartRequired` hints per setting, config-file editing UI round-trip, requirement
  auto-tuning jobs (Prometheus p95) — separate design work.
- Migration of the existing 17 games and billing cutover — deliberately deferred.
