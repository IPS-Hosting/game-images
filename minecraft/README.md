# Minecraft

Manifest-only game definition: Minecraft runs on the external image
[`itzg/minecraft-server`](https://github.com/itzg/docker-minecraft-server), which installs and
updates the server itself (`install.kind: none`). There is no Dockerfile or build workflow in
this directory — only `game.yaml`.

The `runtime.dataPath` (`/data`) follows the itzg volume contract; the container still runs
with the platform's hardened defaults (fixed non-root user, read-only rootfs, dropped
capabilities), which itzg supports.
