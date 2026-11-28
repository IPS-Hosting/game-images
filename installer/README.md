# ipshosting/installer

GitHub: https://github.com/IPS-Hosting/game-images/tree/main/installer

Runs the `steamcmd` and `script` install stages of the game definitions in [games/](../games)
(contract: [gamedef/SPEC.md](../gamedef/SPEC.md) §3.2). The engine starts one short-lived
container per stage with only the data directory of the game server mounted; the game server
container itself never downloads anything.

The image contains:

- steamcmd, bootstrapped at build time, in `$STEAMCMD_DIR` (`/opt/steamcmd`), including
  `linux32/steamclient.so` and `linux64/steamclient.so`
- `steamcmd` on the `PATH`: a wrapper that runs a writable copy of `$STEAMCMD_DIR` (the root
  filesystem is read-only) and retries steamcmd's random exit code 8 ("Missing configuration")
- `curl`, `jq`, `unzip`, `tar`, `gzip`, `xz`, `bzip2`, `procps`
- a non-root user (uid 1000); the engine runs stages with the uid of the game server

The image is rebuilt every week, so that steamcmd and the Debian packages stay current.

## Example

```sh
docker run --rm --read-only --tmpfs /tmp -v "$PWD/server:/home/container" ipshosting/installer:v1 \
  steamcmd +force_install_dir /home/container +login anonymous +app_update 896660 validate +quit
```

## Build

BuildKit cannot bootstrap steamcmd (every download fails with "http error 0"), so the image is
built with podman (or `DOCKER_BUILDKIT=0 docker build`):

```sh
podman build -t ipshosting/installer:v1 installer/
```
