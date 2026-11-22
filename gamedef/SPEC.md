# Game Definition v1: Specification

Normative specification of the IPS Hosting game definition (`games/<short>/game.yaml`, `apiVersion: 1`).

- Schema: [`schema/game-definition.v1.json`](schema/game-definition.v1.json) (JSON Schema 2020-12).
- Reference implementation: this Go module (`github.com/ips-hosting/game-images/gamedef`). The same code
  parses, validates and renders definitions in the catalog CI, in the platform API (schema) and in the
  game daemon. There is exactly one implementation of the semantics below.
- CLI: `go run ./cmd/gamedef validate -root ..` · `render [-set key=value] [-install] <file>` · `schema`.

## 1. Purpose and principles

A definition is the complete, product-independent description of one game. Adding a game means adding
one file to this repository: no daemon, API or frontend release, no database or configuration change.

- **Data, not code.** Everything a game needs is declared. Game-specific logic that cannot be expressed
  declaratively runs as a *stage* or *hook* in a container (§3.2), never as code in the daemon.
- **One engine.** The daemon starts every game the same way: it renders a *start plan* (§4) and hands it
  to `ips-init`, a small static binary that is mounted read-only into every game container, applies the
  file operations inside the container and execs the game.
- **Versioned compatibility.** `requires.engine` names the minimum engine version. Engines reject
  definitions that need a newer engine (`ErrUnsupportedEngine`). The platform keeps serving the last
  compatible revision to older daemons. Within one engine version, changes are additive only.
- **No commercial data.** Prices derive from the platform resource plan and `requirements`.

## 2. Identity and catalog

| Field | Notes |
|---|---|
| `apiVersion` | `1` |
| `short` | `^[a-z0-9]{1,32}$`, unique, equal to the directory name. The universal key. |
| `name`, `description` | Localized text (§2.1). If `description` is missing, the platform fills it from Steam (`steamAppId`). |
| `steamAppId` | Store app id of the game (not the dedicated server). |
| `image` | Logo/cover URL. If missing, the platform image is kept. |
| `tags` | 0–6 values from the controlled vocabulary (schema `$defs.tag`). |
| `source` | Provenance: `from` (`manual`, `egg`, `amp`, `lgsm`), `ref` (upstream file and commit), `license` (SPDX). Imports keep the upstream attribution. |

### 2.1 Localized text

Localized text is `{ de, en? }`. A plain string is shorthand for `{ de: <string> }`. German is required.
English is optional and falls back to German. Applies to names, descriptions, setting labels, port
names and precondition messages.

## 3. Runtime and install

### 3.1 `runtime`

- `image`: container image reference, may be a template.
  - Images from the platform's own repositories (`ipshosting/*`) are referenced by tag.
  - External images must come from an allowed registry (`AllowedImagePrefixes`: pelican/parkervcp
    yolks, itzg, docker library) and must be pinned by digest (`name:tag@sha256:…`). Renovate keeps the
    digests current, and every bump is a reviewed PR.
- `dataPath` (default `/home/ips-hosting`): mount point of the persistent data volume.
  - Egg-based definitions use `/home/container`.
  - The container always runs as the platform's unprivileged user with hardened defaults (read-only root
    filesystem, dropped capabilities, no-new-privileges).
- `env`: map of name to template.
  - `TZ` and `HOME` (= `dataPath`) and all `IPS_*` variables are reserved for the engine.
- `mounts`: binds a file or directory of the data directory (`source`) to an absolute path (`target`),
  read-only by default. `createIfMissing: empty | machineId` creates the source before the first start.

**Engine environment.** Every game and install container gets:
- `TZ`, `HOME`
- `IPS_DATA_PATH`, `IPS_SERVER_ID`, `IPS_SERVER_IP`, `IPS_RAM_MB`, `IPS_CPU_THREADS`
- `IPS_PORT_<ID>` and `IPS_PORT_<ID>_EXPOSED` for every port

### 3.2 `install`: ordered stages

Every stage runs as its own short-lived container with the data directory mounted at `dataPath`, as the
unprivileged user and with network access. Each stage has an optional `when` (§5.3).
Install, update and validate run the same stages. Validate additionally passes `validate` to steamcmd.

| Kind | Fields | Semantics |
|---|---|---|
| `steamcmd` | `appId`, `login` (`anonymous`/`host`), `platform`, `branch`, `branchPassword`, `dir`, `extraArgs` | Runs `steamcmd +runscript <file>`. The engine writes the runscript (`SteamcmdStage.Runscript`) with install dir, login, `app_update`, beta branch and validate. Credentials and branch passwords never appear in argv or the environment. `login: host` uses the host's shared Steam account. |
| `download` | `url` (https), `checksum` (`sha256:<hex>`), `dest`, `extract.format` (`none`, `zip`, `tar`, `tar.gz`, `tar.xz`), `extract.stripComponents` | Static URLs must be pinned by checksum. |
| `script` | `shell`, `env`, `image` | POSIX `sh` script in the installer image (default `ipshosting/installer:v1`, which provides `steamcmd`, `curl`, `jq`, `unzip`, `tar` and `$STEAMCMD_DIR`). The script is **not** a template: values come in via `env`. Imported egg install scripts run here, without root. |
| `container` | `image`, `args`, `env` | Hook container for game-specific logic (mod pipelines, downloaders). Same contract as `script`: data directory in, files out. |

An empty `install` means that the image installs itself (`flags.installable` is false).
`hooks.postInstall` (§5.4) runs after the stages.

## 4. Start

`start` declares either `args` (exec argv, default) or `shell` (`sh -c`), never both. If both are
missing, the image entrypoint runs (external images that own their start).

- **`args`**: list of items.
  - A **plain item** is a template that always produces exactly one argv item, also when it renders
    empty, so an empty password stays an empty argument and never shifts the following flags.
  - The **object form** `{arg, value?, when?}`:
    - It is dropped when `when` is falsy or `arg` renders empty.
    - With `value`, the pair is dropped when the value renders empty; otherwise both items are appended.
    - This is how optional flags are expressed.
- **`shell`**: a POSIX `sh` command line. It is **not** a template. Setting values reach it through
  `start.env`/`runtime.env` (`NAME: '{{ .Setting "name" }}'` then `"$NAME"`), so quoting is the shell's
  job and settings can never inject shell syntax.
- **`env`**: extra environment of the game container (templates).
- **`workingDir`**: relative to `dataPath` (default `dataPath`).

The **start plan** (`gamedef.Render`) is the fully rendered result: image, command, environment,
mounts, config file operations, pre-start hooks, console, stop, ready, query and preconditions.
Nothing in a plan is a template anymore.

## 5. Templates, config files, hooks

### 5.1 Template engine

Templates use Go `text/template` with a restricted context and function whitelist.

**Context:**
- `.Setting "key"` (current value, then default, then `""`), `.SettingBool`, `.SettingInt`, `.SettingList`
- `(.Port "id").Container` / `.Exposed` / `.Protocol`
- `.Server.ID`, `.Server.IPv4`, `.Server.CpuThreads`, `.Server.RamMB`, `.Server.DiskGB`
- `.Timezone`

**Functions:**
- Logic: `and or not len eq ne lt le gt ge`
- Conversion: `default bool boolInt ternary int intf`
- Arithmetic: `add sub mul div max min`
- Strings: `lower upper trim quote replace printf contains hasPrefix hasSuffix`

Template variables (`$x := …`) and `if`/`range`/`with` are allowed. Inside `with` and `range`, dot
is the pipeline value (`{{ with .Port "game" }}{{ .Container }}{{ end }}`) and the context is
reachable through `$` (`{{ $.Setting "name" }}`). `define`, `block` and `template` are not allowed.
Anything else (other functions, fields outside the context, undeclared settings or ports) is a
validation error.

### 5.2 Output rules

- Rendered values must not contain NUL bytes.
- Rendered arguments, environment values, secrets and config values are used verbatim. Only images,
  URLs, paths and branch names are trimmed.
- Values written into line-based config formats must not contain line breaks. This prevents
  injecting additional keys.
- Rendered image references must satisfy §3.1.
- All rendered paths must stay inside the data directory: relative, no `..` escape. Symlinks are
  resolved inside the container by `ips-init`.

### 5.3 Truthiness

`when`, `visibleWhen` and precondition `check`s are rendered and trimmed. Empty, `0` and `false`
(case-insensitive) are falsy, everything else is truthy.

### 5.4 `config`: managed config files

Config files are applied by `ips-init` inside the container before every start (`gamedef.ApplyConfig`),
written atomically.

| Format | Keys | Semantics |
|---|---|---|
| `properties` | `key` | `key=value` (also reads `key: value`) |
| `kv` | `key` | `key<delimiter>value`, `delimiter` default `=` |
| `ini` | `Section.key` (split at the last dot, so `/Script/Engine.GameSession.MaxPlayers` works), `key` = global | missing sections are appended |
| `json` | dot path (`a.b.c`) | numbers/booleans become literals unless the existing value is a string |
| `yaml` | dot path | comments and order are preserved, types as for json |
| `template` | – | full file render, overwrites (escape hatch) |

Rules for all formats except `template`:
- Every occurrence of a managed key gets the managed value.
- Later duplicates of a managed key are removed.
- Unmanaged keys, comments and order are preserved.
- Missing keys are appended.
- Managed keys are the single source of truth: manual edits to them are overwritten on the next start.

Options:
- `onlyIfExists` skips files that the game creates on its first start.
- `createFrom` copies a template file first.
- `when` makes the file conditional.

### 5.5 `hooks`

Declarative file operations: `postInstall` runs after the install stages, `preStart` runs before every
start, both inside the container. Each operation has an optional `when`.
- `copyIfMissing {from, to}`
- `symlink {target, link}`
- `replace {path, pattern (RE2), replacement}`
- `ensureDir {path}`
- `writeFile {path, content, mode}`

## 6. Console, stop, ready, players, query

- **`console.via`**: `stdin` (default), `rcon` (`port`, `passwordSetting`: HIDDENSTRING) or `none`.
  It carries console commands and command-based stops.
- **`stop`**:
  - `via: signal` (default) with `signal` (default `SIGTERM`).
  - `via: command`: `command` is sent over the console channel.
  - `timeoutSeconds` (default 10), after which the server is killed.
- **`ready.pattern`**: RE2 regex on console lines. It marks the transition starting → running and also
  drives the boot test.
- **`players.joinPattern` / `leavePattern`**: count players from console lines when no query protocol fits.
- **`query.protocol`**:
  - `a2s` (`port`)
  - `minecraft` (`port`, optional `fallbackPort` for SLP)
  - `rcon` (`port`, `passwordSetting`, `command` + `pattern` with named groups `name`, `map`, `players`,
    `maxPlayers`, or `countLinesCommand` + `countLinesSkip`)
  - `http` (`url` template, `insecure`, JSON dot `paths`)
  - `none` (default)

  The query drives player counts, freeze detection and server name sync.

## 7. Ports, settings, permissions, preconditions

- **`ports`** map to platform port rows:
  - `identifier` (`^[a-z][a-z0-9_]{0,31}$`, unique), `protocol` (`TCP`/`UDP`), `default`, `group` and
    `static`.
  - Ports of one group keep their spacing during allocation. `default`+`protocol` must be unique.
  - The generated environment names (`IPS_PORT_<ID>`, `IPS_PORT_<ID>_EXPOSED`) must be unique, so
    `game` and `game_exposed` cannot both be declared.
- **`settings`** map to platform setting rows:
  - `key` must be unique **case-insensitively**, because the platform database compares keys
    case-insensitively.
  - `type`: `STRING`, `HIDDENSTRING`, `MULTILINESTRING`, `NUMBER`, `BOOLEAN` (`'0'`/`'1'`),
    `STRINGENUM`, `NUMBERENUM`, `STRINGLIST` (JSON array).
  - `default`, `validation` (validatorjs), `category` (platform category name), `sortOrder`.
  - `values` (`{value, label}`) are required for enums unless `valuesSource` names a platform value
    source.
  - `visibleWhen` (template, evaluated by the panel), `userEditable`.
  - `HIDDENSTRING` is the only way to pass secrets.
- **`filePermissions`**: `match` glob, tri-state `allowRead`/`allowWrite`, `priority`.
- **`backupIgnore`**: globs excluded from backups.
- **`preconditions`**: `check` template, `message`, `severity` (`error` blocks the start, `warning` is
  shown in the panel). They replace hardcoded platform alerts (EULA, tokens, password rules).

## 8. Workshop, CurseForge, flags, requirements

- **`workshop`**: `settings` (STRINGLIST/STRING keys holding item or collection ids), `collections`,
  `appId`, `login`, `dir`.
- **`curseforge.gameId`**: enables CurseForge mod browsing.
- **`flags`**:
  - `renderLogo`, `steamConnect`, `showMap`
  - `showPlayerCount` (default true)
  - `installable` (default: install stages exist; must agree with them)
  - `consoleCommands` (default: console channel exists)

  They replace the frontend's hardcoded game lists.
- **`requirements`**: `minCpuThreads`, `minRam` (MB), `minDiskspace` (GB). Ordering, downgrades and game
  switches are gated by them.

## 9. Validation

1. **JSON Schema** (strict mode): structure, types, enums, unknown fields rejected.
2. **Semantic checks** (`Definition.validate`):
   - duplicate or colliding ports and setting keys
   - enum values and type-correct defaults
   - references to settings and ports (console, query, workshop, templates)
   - HIDDENSTRING for passwords
   - installable and stage consistency
   - RE2 patterns compile
   - static path escapes
   - reserved environment variables
   - template whitelist
   - checksum for static downloads
3. **Default render:** the definition must render (start and install) with all settings at their defaults.

`Lenient` mode (used by engines) skips the schema and tolerates unknown fields, but keeps all checks
this engine understands.

## 10. Repository and CI

- `games/<short>/game.yaml`: one directory per game.
- The `gamedef` workflow runs `go test` (including a catalog test) and `gamedef validate` on changes to
  `games/**` and `gamedef/**`.
- Boot tests (install → start → ready → query → stop) run on a platform game host and gate the platform
  lifecycle `testing` → `published`. They are not part of this workflow.
