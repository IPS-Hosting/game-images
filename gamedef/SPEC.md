# Game Definition v1: Specification

Normative specification of the IPS Hosting game definition (`games/<short>/game.yaml`, `apiVersion: 1`).

- Schema: [`schema/game-definition.v1.json`](schema/game-definition.v1.json) (JSON Schema 2020-12, strict).
- Generated tables: [`schema/engine-features.v1.json`](schema/engine-features.v1.json) (§1.1),
  [`schema/categories.v1.json`](schema/categories.v1.json) (§7.2). Written by `go generate`; a test fails when they are stale.
- Reference implementation: this Go module (`github.com/ips-hosting/game-images/gamedef`), used by the catalog CI,
  the game daemon and `ips-init`. The platform API re-implements only conditions (§5.3), value validation (§7.2),
  the engine table (§1.1) and flag derivation (§8) and must follow this document exactly.
- CLI: `go run ./cmd/gamedef validate -root ..` · `render [-set key=value] [-install] [-validate] <file>` ·
  `images -root ..` (digest-pinned image references of the catalog) · `schema`.

## 1. Purpose and principles

A definition is the complete, product-independent description of one game. Adding a game means adding one
file to this repository: no daemon, API or frontend release.

- **Data, not code.** Game-specific logic that cannot be declared runs as a `script` stage (§3.2) or a hook
  (§5.5) in a container, never as code in the daemon.
- **One engine.** The daemon renders a *start plan* (§4) and hands it to `ips-init`, a small static binary
  mounted read-only into every game container. It applies the file operations inside the container and
  execs the game.
- **Templates only for values.** Go templates (§5.1) appear only in value fields: `run.exec`, every `env`,
  steamcmd `branch`, config `values`/`template`, hook paths, `replace.replacement` and `writeFile.content`.
  Everything that decides *whether* something happens is a structured condition (§5.3). Images, URLs,
  install paths and extra args are literals.
- **No commercial data.** Prices derive from the platform resource plan and `requirements`.

### 1.1 Versioning

`apiVersion: 1` is the only format version. Engines parse **strictly**: unknown fields are an error
(`gamedef.Parse`). The minimum engine version of a definition is computed, not declared:

- `engine-features.v1.json` is `{"baseline": n, "features": [{"path": "/run/tmpfs", "engine": n}, ...]}`.
- `path` is a JSON pointer pattern over the source document; `*` matches exactly one segment (array
  index or map key); segments use RFC 6901 escaping (`~1` = `/`, `~0` = `~`).
- A feature applies when the document contains a non-null value whose pointer has the same number of
  segments and matches segment by segment. Presence counts, also `false`.
- `MinEngine(d) = max(baseline, engine of every applying feature)` (`gamedef.MinEngine`). It is defined on
  the source document only; for a `Definition` built in code (not from `Parse`) it returns `EngineVersion`.
- Adding a field within apiVersion 1 adds a table entry with the next engine version. The platform stores
  the result per revision and serves a revision only to daemons with `EngineVersion >= engine`.

## 2. Identity and catalog

| Field | Notes |
|---|---|
| `apiVersion` | `1` |
| `short` | `^[a-z0-9]{1,32}$`, unique, equal to the directory name. The universal key. |
| `name`, `description` | Localized text (§2.1). Without `description` the platform fills it from Steam. |
| `steamAppId` | Store app id of the game (not the dedicated server). |
| `image` | Logo/cover URL. If missing, the platform image is kept. |
| `tags` | 0–6 unique values from the vocabulary (`gamedef.Tags`, schema `$defs.tag`). |
| `sources` | Provenance list (§2.2). |

### 2.1 Localized text

`{ de, en? }`; a plain string is shorthand for `{ de: <string> }`. German is required, English falls back
to German. Texts must not be blank.

### 2.2 Sources

Each entry: `kind` (`manual`, `egg`, `amp`, `lgsm`), `ref` (upstream file and commit), `license` (SPDX id),
`converter` (`name@version`), `importedSha256` (sha256 of the unedited converter output, base of 3-way
re-imports). Non-manual sources require `ref` and `license`. Upstream license texts are kept in the
repository's `THIRD_PARTY_NOTICES.md`, one section per upstream repository.

## 3. Run and install

### 3.1 `run`

| Field | Notes |
|---|---|
| `image` | Literal image reference. Exactly one of `image` and `imageVariants`. |
| `imageVariants` | `{setting, images: {value: image}}`. `setting` is a STRINGENUM with declared `values` (no `valuesSource`); the map keys equal its values exactly. |
| `dataPath` | Mount point of the data volume, default `/home/ips-hosting` (egg-based definitions: `/home/container`). Clean, absolute, not `/`, outside `/ips`. |
| `workingDir` | Literal, relative to `dataPath` (default `dataPath`). |
| `env` | Name → template. `TZ`, `HOME` and `IPS_*` are reserved; loader variables (`LD_*`, `DYLD_*`) must be literals (also in script stages). |
| `machineId` | `true`: the engine creates a stable random id once in the server's private directory and binds it read-only to `/etc/machine-id`. Definitions cannot bind-mount anything else. |
| `tmpfs` | Up to 8 unique, clean absolute paths that get a fresh tmpfs per start (the root filesystem is read-only; Wine/Xvfb need e.g. `/tmp`). Not `/`, not overlapping `/ips`, `dataPath`, `/etc/machine-id`, `/etc/passwd`, `/etc/group`, `/proc`, `/sys` or `/dev`. Mounted `exec,nosuid,nodev` (games load native libraries from `/tmp`), at most 512 MiB each, counted against the memory limit. Without a tmpfs, `/tmp` is a private, persistent directory of the server. |
| `exec` / `shell` | Start command (§4). |
| `console`, `stop`, `ready` | §6. |

**Images.** Own repositories (`OwnImagePrefixes`: `ipshosting/*`, `ghcr.io/ips-hosting/*`, including mirrors)
are referenced by tag. External images must come from `AllowedImagePrefixes` (pelican/parkervcp yolks, itzg,
docker library) and be pinned by digest (`name:tag@sha256:…`). Renovate keeps digests current. The container
runs unprivileged with hardened defaults (read-only root, dropped capabilities, no-new-privileges).

**Paths.** All definition paths are relative to the data directory. `ips-init` resolves them inside the
container, confined to the data directory with `os.Root`, so neither `..` nor symlinks can escape.

**Engine environment** of every game and install container: `TZ`, `HOME` (= `dataPath`; steamcmd stages
§3.2), `IPS_DATA_PATH`, `IPS_SERVER_ID`, `IPS_SERVER_IP`, `IPS_RAM_MB`, `IPS_CPU_THREADS`, and
`IPS_PORT_<ID>` / `IPS_PORT_<ID>_EXPOSED` per port.

### 3.2 `install`: ordered stages

Each stage runs as its own short-lived container (default image `ipshosting/installer:v1`; the stage command
replaces the image entrypoint, so any image can host a stage) with the data
directory mounted at `dataPath`, unprivileged and with network access. Each stage has an optional `when`
condition (§5.3). Install, update and validate run the same stages; validate adds `validate` to steamcmd.
An empty `install` means that the image installs itself. `hooks.postInstall` runs after the stages.

| Kind | Fields | Semantics |
|---|---|---|
| `steamcmd` | `appId`, `login` (`anonymous`/`host`), `platform`, `branch` (template) or `beta`, `steamSdk`, `dir` (literal), `extraArgs` (literals); no `env` | Runs `steamcmd +runscript <file>`; the engine writes the runscript (`SteamcmdStage.Runscript`). Credentials and branch passwords never appear in argv or the environment. A rendered branch must match `^[A-Za-z0-9._-]{1,64}. |
| `download` | `url` (literal https), `checksum` (required, `sha256:<hex>`; always present in the plan, engines refuse a stage with an empty checksum), `dest`, `extract.format` (`none`, `zip`, `tar`, `tar.gz`, `tar.xz`), `extract.stripComponents` | Verified download. |
| `script` | `shell`, `env`, `image` (literal) | POSIX `sh` script. The installer image provides `steamcmd`, `curl`, `jq`, `unzip`, `tar`, `$STEAMCMD_DIR`. `shell` is not a template: values come in via `env`. |

steamcmd options:
- `beta: {branchSetting, passwordSetting?}`: the branch is the trimmed value of `branchSetting` (STRING or
  STRINGENUM) unless it is empty or `public` (case-insensitive); the password (HIDDENSTRING) is used only
  with a branch. Mutually exclusive with `branch`.
- `steamSdk: true`: after the install, `ips-init` links `<dir>/linux64/steamclient.so` to
  `.steam/sdk64/steamclient.so` and `<dir>/steamclient.so` (Steam app directories keep the 32-bit library
  at the top level) to `.steam/sdk32/steamclient.so`. The install plan carries these as generated
  `ensureDir` + `symlink` post-install operations, before the definition's own `hooks.postInstall`.
- The installer image's `steamcmd` retries exit code 8 ("Missing configuration", frequent with a fresh
  `HOME`) up to two times; a stage fails only when the last run fails.
- `HOME` of a steamcmd stage is `/ips/steam-home`: a private per-stage directory outside the data directory
  (Steam config and token cache), deleted after the stage.

**Host login isolation.** A definition with a `login: host` steamcmd stage or `workshop.login: host` must not
have `script` stages, so customer-influenced files never run next to the host's Steam credentials.

## 4. Start

`run.exec` (argv, no shell) and `run.shell` (`sh -c`) are mutually exclusive. If both are missing, the image
entrypoint runs.

- **`exec`** items:
  - A **plain item** is a template that always produces exactly one argv item, also when it renders empty.
  - The **object form** `{arg, value?, when?}` is dropped when `when` is false or `arg` renders empty. With
    `value`, the pair is dropped when the value renders empty, otherwise both items are appended.
- **`shell`** is **not** a template. Setting values reach it through `run.env`
  (`NAME: '{{ .Setting "name" }}'`, then `"$NAME"`), so settings can never inject shell syntax.

The **start plan** (`gamedef.Render`) is the fully rendered result: image, command, environment, `machineId`,
`tmpfs`, config files, pre-start hooks, console, stop, ready, players, query and evaluated preconditions.
The **install plan** (`gamedef.RenderInstall`) holds the stages and post-install operations. Nothing in a
plan is a template.

## 5. Templates, conditions, config files, hooks

### 5.1 Template engine

Go `text/template` with a restricted context and function whitelist.

- **Context:** `.Setting "key"` (current value, then default, then `""`), `.SettingBool`, `.SettingInt`,
  `.SettingList`; `(.Port "id").Container` / `.Exposed` / `.Protocol`; `.Server.ID`, `.Server.IPv4`,
  `.Server.CpuThreads`, `.Server.RamMB`, `.Server.DiskGB`; `.Timezone`.
- **Functions:** `and or not len eq ne lt le gt ge` · `default bool boolInt ternary int intf` ·
  `add sub mul div max min` · `lower upper trim quote replace printf contains hasPrefix hasSuffix`.

Variables and `if`/`range`/`with` are allowed; `range` only over `.SettingList "key"`. Inside `with`/`range` dot is the pipeline value and the
context is reachable through `$`. `define`, `block` and `template`, other functions, fields outside the
context and undeclared settings or ports are validation errors.

### 5.2 Output rules

- `Render`/`RenderInstall` reject undeclared settings and run `ValidateValue` (§7.2) on every supplied value.
- Rendered values must not contain NUL bytes and are used verbatim (paths and branches are trimmed).
- Limits: one rendered field at most 64 KiB (`MaxRenderedField`), all rendered text of a plan at most 1 MiB
  (`MaxRenderedPlan`).
- Config values must not contain line breaks (no injected keys).
- Rendered paths must be relative and must not escape the data directory.

### 5.3 Conditions

Used by `visibleWhen`, `preconditions[].check` and every `when` (stages, exec items, config files, hooks).
A condition is an object with **exactly one operator**:

| Form | True when |
|---|---|
| `{setting: k, equals: "v"}` / `{setting: k, notEquals: "v"}` | value `==` / `!=` v (exact, case-sensitive) |
| `{setting: k, in: ["a", "b"]}` | value is one of the list (non-empty) |
| `{setting: k, truthy: true\|false}` | truthiness of the value equals the operand |
| `{setting: k, minLength: n}` / `{setting: k, maxLength: n}` | length in Unicode code points `>= n` / `<= n` |
| `{setting: k, matches: "re"}` | RE2 regex matches anywhere in the value (anchor with `^…$`); an invalid pattern is false |
| `{setting: k, notContains: other}` | the value of `other` does not contain the value of `k`, case-insensitively; an empty value of `k` is always true |
| `{all: [c, …]}` / `{any: [c, …]}` / `{not: c}` | non-empty conjunction / disjunction / negation; no `setting` |

- The value of a setting is its current value, else its default, else `""`. Comparisons are on the string form.
- **Truthiness:** the trimmed value is false when empty, `0` or `false` (case-insensitive), otherwise true.
  BOOLEAN settings hold `'0'`/`'1'`.
- Referenced settings must be declared; nesting depth is at most 8.
- Implementations: `gamedef.EvalCondition(cond, settings)` in Go; the platform evaluates the same JSON shape.

### 5.4 `config`: managed config files

Applied by `ips-init` inside the container before every start (`gamedef.ApplyConfig`), written atomically.
`path` and `createFrom` are literals.

| Format | Keys | Semantics |
|---|---|---|
| `properties` | `key` | `key=value` (also reads `key: value`); values are escaped for `java.util.Properties` (`\` → `\\`, leading whitespace), so the game reads exactly the rendered value |
| `kv` | `key` | `key<delimiter>value`, `delimiter` (kv only) default `=` |
| `ini` | `Section.key` (split at the last dot), `key` = global | missing sections are appended; section headers may carry a trailing `;`/`#` comment |
| `json` | dot path, `\.` for a literal dot (`a.b\.c`) | value type from the plan's type hint (below); later duplicates of a managed key are removed |
| `yaml` | as json | comments and order preserved, types as for json; documents whose managed path crosses an anchor, alias or merge key are refused; strings that YAML 1.1 reads as bool/null (`yes`, `on`, `~`, …) are quoted |
| `lines` | line prefix | every line starting with the prefix is replaced by the value (the full line); the longest matching prefix wins; missing lines are appended. The rendered line must start with its prefix and must not start with a longer managed prefix. |
| `xml` | element path from the root, `.`-separated: `A.B.C` (text) or `A.B@attr` (attribute); any step may carry one predicate `name[attr=value]`, e.g. `ServerSettings.property[name=ServerName]@value` | sets the text or attribute of the first element matching every step (a predicate step matches only elements whose attribute has exactly that value); missing elements (and the root of an empty file) are created, with the predicate attribute set. Predicate values may contain `.` and `@` but not `]`; an element with child elements cannot be set. Comments, processing instructions, whitespace (also before `/>`) and CRLF line endings are preserved, self-closing tags are kept. A UTF-8 BOM is kept, whitespace between it and an XML declaration is removed. Values must be valid UTF-8 and XML 1.0 characters. |
| `template` | – | full file render, overwrites (escape hatch) |

For key/value formats every occurrence of a managed key gets the value, later duplicates are removed,
unmanaged content is preserved and missing keys are appended. Applying the same values twice yields the same
file. Options: `onlyIfExists` (skip files the game creates on first start), `createFrom` (copy a template
file first, only when the file is missing), `when`.

**JSON/YAML types.** `PlanConfigFile.types` maps a key to `string`, `number` or `bool`, derived from the
value template: a literal by its own syntax (`8`, `false`, else string); a single `.Setting` of a NUMBER or
NUMBERENUM setting, `.SettingInt`, a port or a numeric `.Server` fact is a number; `.SettingBool` is a bool;
a single `.Setting` of a STRING, HIDDENSTRING, STRINGENUM or STRINGLIST setting and `.Server.IPv4` are
**always strings**, also when the existing value is a number. Without a hint (BOOLEAN via `.Setting`,
expressions, mixed text) the type of the existing value is kept; a new value is a string. A rendered value
without the syntax of its type (e.g. an empty number) is written as a string.

**Order before every start:** `ips-init` applies the `preStart` hooks first, then the config files (both in
list order), so hooks can create directories and default files that config then manages. Missing parent directories of
config files are created.

### 5.5 `hooks`

`postInstall` runs after the install stages, `preStart` before every start, both inside the container. Each
operation has an optional `when` and exactly one of: `copyIfMissing {from, to}`, `symlink {target, link}`,
`replace {path, pattern (RE2), replacement}`, `ensureDir {path}`, `writeFile {path, content, mode, ifMissing}`.
`replacement` is literal (no `$1` expansion). `writeFile.ifMissing: true` writes only when the file does not exist.

## 6. Console, stop, ready, players, query

- **`run.console.via`**: `stdin` (default), `rcon` (`port`, `passwordSetting`: HIDDENSTRING) or `none`.
- **`run.stop`**: `via: signal` (default, `signal` default `SIGTERM`) or `via: command` (`command` over the
  console channel); `timeoutSeconds` (1–600, default 10), then the server is killed.
- **`run.ready.pattern`**: RE2 regex on console lines; marks starting → running and drives the boot test.
- **`players.joinPattern` / `leavePattern`**: player counting from console lines.
- **`query.protocol`** (every protocol except `none` needs `port`; fields of other protocols are errors):
  - `a2s`; `minecraft` (optional `fallbackPort`)
  - `rcon`: `passwordSetting`, `command` + `pattern` (named groups `name`, `map`, `players`, `maxPlayers`) or
    `countLinesCommand` + `countLinesSkip`
  - `http`: `path` (literal, starts with `/`), `scheme` (`http` default, `https`), `insecure` (https only),
    JSON dot `paths` (`name`, `map`, `players`, `maxPlayers`). The engine builds
    `scheme://<server IPv4>:<exposed port><path>`; the host is never configurable.
  - `none` (default)

## 7. Ports, settings, files, preconditions

### 7.1 Ports

`identifier` (`^[a-z][a-z0-9_]{0,31}$`, unique), `protocol` (`TCP`/`UDP`), `default`, `group`, `static`.
Ports of one group keep their spacing during allocation. `default`+`protocol` must be unique, and the
generated environment names must be unique (`game` and `game_exposed` cannot both exist).

### 7.2 Settings

- `key`: `^[a-zA-Z][a-zA-Z0-9_.-]{0,127}$`, unique **case-insensitively**.
- `type`: `STRING`, `HIDDENSTRING` (the only way to pass secrets), `MULTILINESTRING`, `NUMBER`, `BOOLEAN`
  (`'0'`/`'1'`), `STRINGENUM`, `NUMBERENUM`, `STRINGLIST` (JSON array of strings).
- `default` (string), `values` (`{value, label}`, enums only) or `valuesSource` (platform value source),
  `visibleWhen` (condition), `userEditable`.
- `category`: an id of `categories.v1.json` (`general` default, `network`, `gameplay`, `world`, `admin`,
  `mods`, `steamBeta`, `advanced`, `performance`); the file holds the de/en labels. Order within a
  category is the list order.
- `generate: {length (4–256), charset: alnum (default) | hex}`: STRING/HIDDENSTRING only, without default (or
  `''`); the platform fills a random value at server creation when the value is empty.
- `validation: {required?, min?, max?, integer?, minLength?, maxLength?, pattern?}`, checked by
  `gamedef.ValidateValue(setting, value)`:
  0. Every value must be valid UTF-8 without NUL; only MULTILINESTRING may contain `\r`/`\n`.
  1. Empty value: invalid if `required`, otherwise valid (no further rules).
  2. Type: BOOLEAN `'0'`/`'1'`; NUMBER/NUMBERENUM match `^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`; STRINGLIST a JSON string array; enums with
     `values` one of them.
  3. `integer` (no fraction, no exponent), `min`, `max` (numeric types only) on the number; `minLength`, `maxLength` (code points) and
     `pattern` (RE2, must match the **whole** value) on string types only.
  Non-empty defaults must pass `ValidateValue`. MULTILINESTRING settings may not be referenced in config
  `values` (only in `template` files, env and args).

### 7.3 Files and preconditions

- **`protectedFiles`**: `[{path (glob, relative to the data directory, leading / allowed), readOnly?: true,
  hidden?}]`, at least one flag true. Panel file-manager guard rails, not a security boundary. Earlier
  entries take precedence (platform: hidden → no read/write, readOnly → no write, priority = list index).
- **`backupIgnore`**: globs excluded from backups.
- **`preconditions`**: `check` (condition), `message`, `severity` (`error` blocks the start, `warning` is
  shown in the panel). `StartPlan.Blocking()` returns the failed errors.

## 8. Workshop, flags, requirements

- **`workshop`**: `settings` (STRINGLIST/STRING keys with item or collection ids), `collections`, `appId`,
  `login` (`anonymous` default, `host` see §3.2), `dir`.
- **Flags** are derived (`gamedef.EffectiveFlags`); `flags` only overrides `showMap`, `showPlayerCount`,
  `steamConnect`, `consoleCommands`:

  | Flag | Derived value |
  |---|---|
  | `installable` | `install` is non-empty (not overridable) |
  | `showMap` | query `a2s`, or `rcon` with a `pattern` that has a named group `map`, or `http` with `paths.map` |
  | `showPlayerCount` | query protocol is not `none`, or a `players` pattern is set |
  | `steamConnect` | `steamAppId` is set and query is `a2s` |
  | `consoleCommands` | `run.console.via` is not `none` (an override `true` requires a console channel) |

- **`requirements`**: `minCpuThreads`, `minRam` (MB), `minDiskspace` (GB). Ordering, downgrades and game
  switches are gated by them.

## 9. Validation

1. **JSON Schema** (CI and platform API, package `gamedef/schema`, `schema.ValidateSchema`): structure,
   types, enums, unknown fields.
2. **Parse** (`gamedef.Parse`, every consumer): strict decoding (unknown fields rejected, JSON types as in the
   schema: no `1` for a string, `yes` for a bool or `null` for an object) plus the schema's size and range limits
   (`TestSchemaAndParseAgreeOnMutations` keeps both in sync), plus structural and
   semantic checks without the schema library: required fields and enums, references to settings and ports,
   setting types of references, defaults and validation rules, conditions, images and digests, literal
   fields without templates, path escapes, tmpfs rules, reserved environment variables, host-login
   isolation, query protocol fields, template whitelist.
3. **Default render:** start and install plans must render with all settings at their defaults.

## 10. Repository and CI

- `games/<short>/game.yaml`: one directory per game.
- The `gamedef` workflow runs `go test` (including the catalog test and the generated-file check),
  `gamedef validate` and `gamedef images -check` on changes to `games/**` and `gamedef/**`.
- PR CI checks schema, semantics, default render and the existence of pinned images (registry v2 API, a
  missing own tag is a warning), all without pulling. Boot tests (install → start → ready → query → stop)
  run on platform game hosts as part of the platform lifecycle (`testing` → `published`) and on image or
  build changes, not in GitHub Actions.
