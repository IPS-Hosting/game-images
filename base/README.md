# IPS Hosting base image

Shared base image for all IPS Hosting game server images.

Provides: non-root user `ips-hosting` (UID/GID 4711), workdir and volume
`/home/ips-hosting`, `HOME` env var, `ca-certificates` and `tzdata`.

Game-specific dependencies, exposed ports and the entrypoint are added by the
per-game Dockerfiles that build `FROM ipshosting/base:<version>`.
