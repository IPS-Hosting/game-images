# ARK: Survival Evolved

Dedicated server for ARK: Survival Evolved. Allows connection from Steam players and Epic Games players (if `PUBLIC_IP` is set).
Xbox, PS4, and PC players that play the game using Microsoft store (Xbox game pass), are not able to join the server.

GitHub: https://github.com/IPS-Hosting/game-images/tree/main/ark

## Basic usage

For advanced usage, refer to https://docs.docker.com

```shell
# Create the docker container
docker create -it --restart always \
  --name ark-server \
  -p 27020:27020/tcp \
  -p 27015:27015/udp \
  -p 7777:7777/udp \
  -p 7778:7778/udp \
  ipshosting/game-ark:v2

# Start the server
docker start ark-server

# Stop the server
docker stop ark-server

# Restart the server
docker restart ark-server

# View server logs
docker logs ark-server

# Attach to server console to write commands and see output in realtime (de-attach by pressing CTRL-P + CTRL-Q).
docker attach ark-server

# Remove the container
docker rm ark-server
```

## Commands

By default, when starting the container, it will be installed and updated, and the ark Server is started afterwards.
You can create a container with a different command to change this behaviour:

- **update** Only install the latest version of the server. It won't be started and the container will exit after the ark server is installed and updated.
- **update_validate** Same like update but will also validate the files. Recommended for the initial installation of the server.
- **start** Only start the ark server without installing or updating.

## Data persistence

Game server data is kept in `/home/ips-hosting`.
By default a volume will be auto-created which will persist the game server data across server restarts.
When you re-create the container, a new volume is created and you can't access the old data unless you manually mount the old volume.
See https://docs.docker.com/storage/volumes/ for more information.

To persist the game server data on the host filesystem, use `-v /absolute-path/on/host:/home/ips-hosting` when creating the docker container.

## Ports

- 27020/tcp (rcon)
- 27015/udp (query)
- 7777/udp (game)
- 7778/udp (rawSocket) - always game+1

You can change the port with the `GAME_PORT`, `QUERY_PORT` and `RCON_PORT` environment variables.

## Env variables

Env variables can be configured with the `-e "KEY=VAL"` flag when creating the container. The flag can be used multiple times.
To change the env variables, you need to re-create the container.

### update and update_validate

The following env variables are available during `update` and `update_validate`.

`BETA_BRANCH` Used to download a different branch of the server.

`BETA_PASSWORD` The password for the beta branch.

### start

The following env variables are available during `start`.

`MAP` The map the server runs on. Defaults to `TheIsland`.

`HOST` The host the server listens on. Defaults to `0.0.0.0`

`PUBLIC_IP` The public IP of the server (optional). Can be set to allow connections from Epic Games players (crossplay).

`GAME_PORT` The udp game port the server listens on. Defaults to `7777`. Remember to also update the container port bindings when changing this variable.

`QUERY_PORT` The udp port the server listens on for queries. Defaults to `27015`. Remember to also update the container port bindings when changing this variable.

`RCON_PORT` The tcp port the server listens on for rcon. Defaults to `27020`. Remember to also update the container port bindings when changing this variable.

`RCON_ENABLED` Set to `True` to enable rcon and to `False` to disable rcon. Defaults to `True`.

`MAX_PLAYERS` The maximum amount of players that can join the server. Defaults to `10`.

`MODS` A comma seperated list of mod ids, that should be loaded. These are automatically managed (installed and updated) by the ARK server.

`SESSION_NAME` The name of the server.

`RAW_SOCKETS` Set to `true` to enable raw sockets.

`SERVER_PASSWORD` The server password, players need to enter before they can join.

`SERVER_ADMIN_PASSWORD` The server admin password, which is required to issue the `enablecheats` command.

`SERVER_GAME_LOG` Set to `true` to enable server admin logs.

`SERVER_GAME_LOG_INCLUDE_TRIBE_LOGS` Set to `true` to include tribe logs in the game log.

`FORCE_ALLOW_CAVE_FLYERS` Set to `true` to allow flyer dinos in caves.

`ALLOW_FLYER_CARRY_PVE` Set to `true` to allow flyers on PvE to carry wild dinos.

`NO_BATTLEYE` Set to `true` to disable BattlEye anti cheat.
