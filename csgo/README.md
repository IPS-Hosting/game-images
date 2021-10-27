# Counter Strike: Global Offensive

GitHub: https://github.com/IPS-Hosting/game-images/tree/main/csgo

## Basic usage
For advanced usage, refer to https://docs.docker.com
```shell
# Create the docker container
docker create -it --restart always \
  --name csgo-server \
  -p 27015:27015/udp \
  -p 27015:27015/tcp \
  -p 27005:27005/tcp \
  -p 27020:27020/udp \
  -e "GSLT=YOUR_GSLT" \
  ipshosting/game-csgo:v2
  
# Start the server
docker start csgo-server

# Stop the server
docker stop csgo-server

# Restart the server
docker restart csgo-server

# View server logs
docker logs csgo-server

# Attach to server console to write commands and see output in realtime (de-attach by pressing CTRL-P + CTRL-Q).
docker attach csgo-server

# Remove the container
docker rm csgo-server
```

## Commands
By default, when starting the container, it will be installed and updated, and the csgo Server is started afterwards.
You can create a container with a different command to change this behaviour:
* **update** Only install the latest version of the server. It won't be started and the container will exit after the csgo server is installed and updated.
* **update_validate** Same like update but will also validate the files. Recommended for the initial installation of the server.
* **start** Only start the csgo server without installing or updating.

## Data persistence
Game server data is kept in `/home/ips-hosting`.
By default a volume will be auto-created which will persist the game server data across server restarts.
When you re-create the container, a new volume is created and you can't access the old data unless you manually mount the old volume.
See https://docs.docker.com/storage/volumes/ for more information.

To persist the game server data on the host filesystem, use `-v /absolute-path/on/host:/home/ips-hosting` when creating the docker container.

## Ports
* 27015/udp (game)
* 27015/tcp (rcon) always the same as game
* 27005/tcp (client)
* 27020/udp (tv)

You can change the ports with the `GAME_PORT`, `CLIENT_PORT` and `TV_PORT` environment variables.

## Env variables
Env variables can be configured with the `-e "KEY=VAL"` flag when creating the container. The flag can be used multiple times.
To change the env variables, you need to re-create the container.

### update and update_validate
The following env variables are available during `update` and `update_validate`.

`BETA_BRANCH` Used to download a different branch of the server.

`BETA_PASSWORD` The password for the beta branch.


### start
The following env variables are always available during `start`.

`GSLT` The game server login token. This is required to launch the server. Can be generated [here](https://steamcommunity.com/dev/managegameservers).

`HOST` The host address, the server listens on. Defaults to `0.0.0.0`

`GAME_PORT` The game port to use. Defaults to `27015`. Remember to also update the container port bindings when changing this variable.

`CLIENT_PORT` The client port to use. Defaults to `27005`. Remember to also update the container port bindings when changing this variable.

`TV_PORT` The client port to use. Defaults to `27020`. Remember to also update the container port bindings when changing this variable.

`MAX_PLAYERS` The maximum amount of players that can join the server. Defaults to `10`.

`TICKRATE` The tickrate to use. Defaults to `66`.

`GAME_TYPE` and `GAME_MODE` together specify which game mode the server runs. Defaults to Classic Casual (`GAME_TYPE=0` and `GAME_MODE=0`).

Gamemode            | GAME_TYPE | GAME_MODE 
------------------- | :-------: | :-------:
Arms Race           | 1			| 0
Classic Casual      | 0			| 0
Classic Competitive | 0			| 1
Custom              | 3			| 0
Deathmatch          | 1			| 2
Demolition          | 1			| 1
Wingman             | 0			| 2
Danger Zone         | 6			| 0

`MAP_GROUP` The map group to use. Defaults to `mg_active`.

`MAP` The map the server loads after startup. Defaults to `de_mirage`.

`WORKSHOP_START_MAP` The workshop id of a workshop map to load after startup. Requires `HOST_WORKSHOP_COLLECTION` to be set up. Also the collection needs to include the map addon.

`HOST_WORKSHOP_COLLECTION` The id of a workshop collection to load. Requires `AUTHKEY` to be set up.

`AUTHKEY` A Steam Web API key. Can be generated [here](https://steamcommunity.com/dev/apikey). Required to use `HOST_WORKSHOP_COLLECTION`.

`INSECURE` Set to `true` to disable VAC.

`NOBOTS` Set to `true` to disable bots.
