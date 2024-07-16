# ARMA III

GitHub: https://github.com/IPS-Hosting/game-images/tree/main/arma3

## Basic usage

For advanced usage, refer to https://docs.docker.com

```shell
# Create the docker container
docker create -it --restart always \
  --name arma3-server \
  -p 2302:2302/udp \
  -p 2303:2303/udp \
  -p 2304:2304/udp \
  -p 2305:2305/udp \
  -p 2306:2306/udp \
  -e "STEAM_USERNAME=your_steam_username" \
  -e "STEAM_PASSWORD=your_steam_password" \
  ipshosting/game-arma3:v2

# Start the server
docker start arma3-server

# Stop the server
docker stop arma3-server

# Restart the server
docker restart arma3-server

# View server logs
docker logs arma3-server

# Attach to server console to write commands and see output in realtime (de-attach by pressing CTRL-P + CTRL-Q).
docker attach arma3-server

# Remove the container
docker rm arma3-server
```

## Commands

By default, when starting the container, it will be installed and updated, and the arma3 Server is started afterwards.
You can create a container with a different command to change this behaviour:

- **update** Only install the latest version of the server. It won't be started and the container will exit after the arma3 server is installed and updated.
- **update_validate** Same like update but will also validate the files. Recommended for the initial installation of the server.
- **start** Only start the arma3 server without installing or updating.

## Data persistence

Game server data is kept in `/home/ips-hosting`.
By default a volume will be auto-created which will persist the game server data across server restarts.
When you re-create the container, a new volume is created and you can't access the old data unless you manually mount the old volume.
See https://docs.docker.com/storage/volumes/ for more information.

To persist the game server data on the host filesystem, use `-v /absolute/path/on/host:/home/ips-hosting` when creating the docker container.
The container is run as a non-root user by default and the user running inside the container has the id 1000. Make sure that the mounted directory is readable and writable by the user running the container. There are 2 ways to achieve this:

- Change the owner of the host directory: `chown -R 1000 /absolute/path/on/host` OR
- Run the container as the user, which owns the files on the host system. Make sure to specify the id of your local user, because the name is uknown inside the container. You can find it out using `id YOUR_USERNAME`. Then run the docker command using the `--user USER_ID` flag. E.g.: `docker create --user 500 ...`.

## Ports

- 2302/udp (game)
- 2303/udp (query) - always game+1
- 2304/udp (steam) - always game+2
- 2305/udp (VON) - always game+3
- 2306/udp (battlEye) - always game+4

You can change the ports with the `GAME_PORT` environment variable.

## A note about Steam credentials

To be able to download the ARMA III dedicated server, a Steam account which owns the game is required.
These credentials need to be specified during `update` and `update_validate` via env variables (see below).
The account must not have Steam guard enabled. Because of that it is not recommended to use your personal Steam account.

## Env variables

Env variables can be configured with the `-e "KEY=VAL"` flag when creating the container. The flag can be used multiple times.
To change the env variables, you need to re-create the container.

### update and update_validate

The following env variables are available during `update` and `update_validate`.

`BETA_BRANCH` Used to download a different branch of the server.

`BETA_PASSWORD` The password for the beta branch.

`STEAM_USERNAME` The username of a steam account owning ARMA III.

`STEAM_PASSWORD` The password of a steam account owning ARMA III.

### start

The following env variables are always available during `start`.

`MODE` Either `server` or `client` to run as a headless client. Defaults to `server`.

`USE_X64` Set to `true` to enable the use of the 64 bit executable. Defaults to `true`.

`PROFILE` The profile to use. Defaults to `server`.

`LIMIT_FPS` The maximum amount of server FPS. Numeric value between 5 and 1000. Defaults to `50`.

### server mode

The following env variables are available during `start` when in `server` mode.

`HOST` The host address, the server should listen on. Defaults to `0.0.0.0`

`GAME_PORT` The game port to use. Defaults to `2302`. Remember to also update the container port bindings when changing this variable.

`BASIC_CFG` The name of the basic config file. Defaults to `basic.cfg`

`SERVER_CFG` The name of the server config file. Defaults to `server.cfg`

`AUTO_INIT` Set to `true` to automatically initialize the mission on server start.

`LOAD_MISSION_TO_MEMORY` Set to `true` to load the mission into memory once the first client downloads it. This requires more memory, but can save some CPU cycles.

### client mode

The following env variables are available during `start` when in `client` mode.

`GAME_SERVER_IP` The ip of the game server to connect to. This must be set.

`GAME_SERVER_PORT` The port of the game server to connect to. Defaults to 2302.

`GAME_SERVER_PASSWORD` The passowrd of the game server to connect to.

## Mods

All mods should be installed in the subdirectory `mods`.
On every start, all mods in this folder are patched to work on Linux. This means that all files are converted to lower-case and spaces are replaced by underscores.

Mods can be automatically downloaded and kept up to date from the Steam workshop. Therefore you need to provide a space seperated list of steam workshop ids in the `MANAGED_MODS` env variable during `update` or `update_validate`. This will symlink the mods to the `mods` subdirectory.
Note that this will only install the mods but do not load them.

The following environment variables can be used to actually load the mods:

`MODS` A list of mods to load, seperated by semicolons. Each entry should be a relative path to the folder containing the mod. e.g. `mods/@ace;mods/@cba_a3`.
It is important that the mods are all lower-case and spaces are replaced with underscores.

`SERVER_MODS` Same like `MODS`, but used for server-side only mods. Clients will not see these mods. Only works when in the `server` mode.

In addition, when in `server` mode, mod keys can be automatically extracted to the `keys` subdirectory. To enable this, set `EXTRACT_MOD_KEYS=true`.
