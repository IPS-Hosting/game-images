# Counter Strike: Global Offensive

## Basic usage
For advanced usage, refer to https://docs.docker.com
```shell
# Create the docker container
docker create -it --restart always \
  --name gmod-server \
  -p 27015:27015/udp \
  -p 27015:27015/tcp \
  -p 27005:27005/tcp \
  ipshosting/game-gmod:v2
  
# Start the server
docker start gmod-server

# Stop the server
docker stop gmod-server

# Restart the server
docker restart gmod-server

# View server logs
docker logs gmod-server

# Attach to server console to write commands and see output in realtime (de-attach by pressing CTRL-P + CTRL-Q).
docker attach gmod-server

# Remove the container
docker rm gmod-server
```

## Commands
By default, when starting the container, it will be installed and updated, and the gmod Server is started afterwards.
You can create a container with a different command to change this behaviour:
* **update** Only install the latest version of the server. It won't be started and the container will exit after the gmod server is installed and updated.
* **update_validate** Same like update but will also validate the files. Recommended for the initial installation of the server.
* **start** Only start the gmod server without installing or updating.

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

You can change the ports with the `GAME_PORT` and `CLIENT_PORT` environment variables.

## Game content
CSS is automatically downloaded and mounted during `update_validate`.
Content is placed in `/home/ips-hosting/.ips-hosting/game-content/cstrike`.
This is required because many maps use props from CSS. Without the content being mounted on the server, those props have no physics.
Things that are not needed for content like maps are not kept to reduce disk usage.

## Env variables
Env variables can be configured with the `-e "KEY=VAL"` flag when creating the container. The flag can be used multiple times.
To change the env variables, you need to re-create the container.

### update and update_validate
The following env variables are available during `update` and `update_validate`.

`BETA_BRANCH` Used to download a different branch of the server.

`BETA_PASSWORD` The password for the beta branch.


### start
The following env variables are always available during `start`.

`USE_X64` Set to `true` to use the 64 bit executable. Note that, as of writing, the 64 bit executable is only available in several beta branches.

`HOST` The host address, the server listens on. Defaults to `0.0.0.0`

`GAME_PORT` The game port to use. Defaults to `27015`. Remember to also update the container port bindings when changing this variable.

`CLIENT_PORT` The client port to use. Defaults to `27005`. Remember to also update the container port bindings when changing this variable.

`MAX_PLAYERS` The maximum amount of players that can join the server. Defaults to `10`.

`TICKRATE` The tickrate to use. Defaults to `66`.

`MAP` The map the server loads after startup. Defaults to `gm_construct`.

`GAMEMODE` The gamemode the server loads after startup. Defaults to `sandbox`.

`HOST_WORKSHOP_COLLECTION` The id of a workshop collection to load.

`GSLT` The game server login token.
Setting a token has the advantage, that players keep the server in their favorites when the ip changes and the server won't be displayed at the bottom of the server list.
Can be generated [here](https://steamcommunity.com/dev/managegameservers).

`INSECURE` Set to `true` to disable VAC.

`NOBOTS` Set to `true` to disable bots.

`NOWORKSHOP` Set to `true` to disable the loading of all workshop addons. Useful for debugging.

`NOADDONS` Set to `true` to disable the loading of all filesystem addons. Useful for debugging.

`DISABLELUAREFRESH` Set to `true` to disable lua refresh. Lua refresh can be buggy in docker environments. Defaults to `true`.
