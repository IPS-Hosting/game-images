# Wine/Proton runtime contract

Windows-only dedicated servers run under Wine or Proton in the pelican yolk images. This file
describes the contract once; `enshrouded`, `abioticfactor` (Proton) and `icarus`, `sotf`,
`spaceengineers` (Wine) follow it and refer to it from their header. Other Windows-only games should
copy the matching pattern.

Status: none of this has been boot-tested on a game host yet (M3).

## Images

| Runtime | Image (pin by digest) | Launcher |
|---|---|---|
| Wine | `ghcr.io/pelican-eggs/yolks:wine_latest` | `wine ./Server.exe` |
| Proton | `ghcr.io/pelican-eggs/steamcmd:proton` (GE-Proton in `/usr/local/bin`) | `proton run ./Server.exe` |

The image entrypoints are never run: they call steamcmd and winetricks at every start. Everything
they do is declared in the definition instead (steamcmd stages, a one-off `script` stage, `run.shell`).

## Install: steamcmd with `platform: windows`

```yaml
install:
  - kind: steamcmd       # Steamworks redistributable (steamclient64.dll ...), as the yolks install it
    appId: 1007
    platform: windows
  - kind: steamcmd
    appId: <dedicated server app>
    platform: windows
```

`platform: windows` makes the runscript start with `@sSteamCmdForcePlatformType windows`, so
steamcmd downloads the Windows depots on Linux. Under plain Wine the server loads the Windows
`steamclient64.dll` of the redistributable. Under Proton the Steam API is bridged (lsteamclient) to the
Linux `steamclient.so`, which must be current (the redistributable's is too old: "No SteamClient023"):
Proton definitions copy it from the installer's steamcmd with a `script` stage into `.steam/sdk64` and
`.steam/steam/linux64` (boot-tested 2026-11-27).

## Prefix inside the data directory

The prefix always lives in the data directory, so it survives restarts and needs no extra mount:

| Runtime | Environment | Prefix |
|---|---|---|
| Wine | `WINEPREFIX=/home/container/.wine`, `WINEARCH=win64` | `.wine/` |
| Proton | `STEAM_COMPAT_CLIENT_INSTALL_PATH=/home/container/.steam/steam`, `STEAM_COMPAT_DATA_PATH=/home/container/.steam/steam/steamapps/compatdata/<appId>` | `.steam/steam/steamapps/compatdata/<appId>/pfx/` |

- Proton needs the compatdata directory to exist: `hooks.preStart: ensureDir` for it.
- `HOME` is the data directory (engine-reserved). That also holds `~/.cache` and `~/.local` for Wine.
- `WINEDEBUG=-all` and `winemenubuilder.exe=d` in `WINEDLLOVERRIDES` keep the log readable and stop
  Wine from writing desktop entries. Wine keeps the yolk default `mscoree,mshtml=` unless the server is
  a .NET application (Space Engineers: `mscoree=n,b;mshtml=n,b` plus wine-mono).
- Runtime components (winetricks verbs, wine-mono) are installed once by a `script` stage that runs
  in the same Wine image as the game (not the default installer image). winetricks skips installed
  verbs, so install, update and validate can rerun the stage. Its download cache goes to
  `XDG_CACHE_HOME=/tmp/winetricks-cache` (the private `/tmp` of the stage, not the data directory)
  and is removed afterwards. wine-mono comes from a checksummed `download` stage.
- Wine creates a missing prefix on first start; Proton copies its default prefix on first start.
  Prefixes with winetricks verbs (`sotf`, `spaceengineers`) can only be rebuilt by the install stage,
  so their start script fails with a hint to reinstall when `system.reg` is missing.
- Prefixes are in `backupIgnore`: they are large and recreated by install or the first start. Saves
  must never be inside the prefix: use the game's own save path or `-userdatapath`/`-path` options.

## Read-only root: `run.tmpfs`

Every definition declares:

```yaml
run:
  tmpfs: [/tmp]
```

- Wine keeps its wineserver socket in `/tmp/.wine-<uid>/`, Xvfb its lock and socket in `/tmp/.X99-lock`
  and `/tmp/.X11-unix/`. `/tmp` is writable without the tmpfs too (the engine bind-mounts a private,
  persistent tmp directory), but that directory survives restarts. A stale `/tmp/.X99-lock` whose PID
  exists again in the new container makes Xvfb refuse to start, so a fresh tmpfs per start is required.
- Nothing else outside the data directory has to be writable. Xvfb compiles its keymap into
  `/var/lib/xkb` only when that directory is writable and falls back to `/tmp` otherwise. The Proton
  launcher runs from the read-only `/usr/local/bin` and must not need to write there (it already runs
  as a non-root user in the yolk); this is part of the boot test.
- The tmpfs counts towards the container memory limit; Wine and Xvfb keep only a few MB there.

## Display: Xvfb, not `xvfb-run`

Unreal and Unity servers under Wine expect an X display (the yolks start Xvfb for every Wine and
Proton game; the Abiotic Factor egg uses `xvfb-run`). The definitions start Xvfb in the background
and set `DISPLAY` through `run.env`:

```sh
Xvfb :99 -screen 0 1024x768x16 -nolisten tcp >/dev/null 2>&1 &
i=0
while [ ! -e /tmp/.X11-unix/X99 ] && [ "$i" -lt 50 ]; do sleep 0.1; i=$((i + 1)); done
```

`xvfb-run` is not used: it runs the server as a child of a shell script and does not forward the
stop signal, so the server would be killed without saving. Install stages start their own Xvfb on
`:98`.

## Stop: SIGINT, never SIGTERM alone

Wine maps SIGINT to `CTRL_C_EVENT`, which Windows servers handle as "save and quit". Wine does not
translate SIGTERM into a console event, so a plain `docker stop` ends the server without saving.
All definitions use:

```yaml
stop: { via: signal, signal: SIGINT, timeoutSeconds: 60 }   # Space Engineers: 120
console: { via: none }
```

ips-init forwards the stop signal to its direct child only, so the start command must make sure the
server process itself gets SIGINT:

- **Server logs to stdout** (`sotf`, `spaceengineers`): `exec wine ./Server.exe ...`. Wine runs the
  executable in the exec'd process, so it receives SIGINT directly.
- **Server logs only to a file, or runs under Proton** (`enshrouded`, `icarus`, `abioticfactor`): the
  server runs in the background, `tail -c0 -F <log> --pid=<launcher>` copies the log to stdout (for
  the console and `ready.pattern`), and a trap turns SIGINT and SIGTERM into SIGINT for the server
  process. `proton run` is a Python launcher, and Wine starts every Windows process as its own Unix
  process, so the server is found by its executable name in `/proc/*/cmdline` (argv[0];
  `steam.exe` is excluded because its command line contains the server name too). The shell then
  waits for the launcher and for every server process before it exits, so the container does not stop
  while the server is still saving.

```sh
game_pids() {
  for p in /proc/[0-9]*; do
    argv0=$(tr '\0' '\n' 2>/dev/null < "$p/cmdline" | head -n 1)
    case "$argv0" in
      *steam.exe*) ;;
      *Server-Win64-Shipping.exe*) echo "${p#/proc/}" ;;
    esac
  done
}
trap 'for pid in $(game_pids); do kill -INT "$pid" 2>/dev/null; done' INT TERM
proton run ./Server-Win64-Shipping.exe "$@" &
launcher=$!
tail -c0 -F Saved/Logs/Server.log --pid="$launcher" 2>/dev/null &
status=0
while :; do
  wait "$launcher"; status=$?
  kill -0 "$launcher" 2>/dev/null || break
done
while [ -n "$(game_pids)" ]; do sleep 1; done
exit "$status"
```

The trap only works because the shell is the direct child of ips-init: a shell started in the
background ignores SIGINT and cannot trap it. If the server does not stop within `timeoutSeconds`,
the engine kills the container.

## Boot test checklist (M3)

1. Install: both steamcmd stages, the script stage (Wine image as non-root, read-only root, private `/tmp`).
2. Start: prefix created in the data directory, nothing written outside it except `/tmp`.
3. `ready.pattern` matches on stdout (tail for log-file servers).
4. A2S query answers on the query port.
5. Stop: SIGINT reaches the server, the save is written, the container exits before the timeout.
6. Restart after a hard kill: Xvfb starts again (fresh tmpfs), the prefix is reused.
