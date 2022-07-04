# ARMA III for Windows

GitHub: https://github.com/IPS-Hosting/game-images/tree/main/arma3-win

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
  ipshosting/game-arma3-win:v1

# Start the server
docker start arma3-server

# Stop the server
docker stop arma3-server

# Restart the server
docker restart arma3-server

# View server logs
docker logs arma3-server

# Attach to server console to see output in realtime (de-attach by pressing CTRL-P + CTRL-Q).
docker attach arma3-server

# Remove the container
docker rm arma3-server
```

## Commands
By default, when starting the container, it will be installed and updated, and the arma3 Server is started afterwards.
You can create a container with a different command to change this behaviour:
* **update** Only install the latest version of the server. It won't be started and the container will exit after the arma3 server is installed and updated.
* **update_validate** Same like update but will also validate the files. Recommended for the initial installation of the server.
* **start** Only start the arma3 server without installing or updating.

## Data persistence
Game server data is kept in `C:/Users/ContainerUser/arma3server`. In addition to that, the SteamCMD and its data is kept in `C:/Users/ContainerUser/steamcmd`.
By default 2 volumes will be auto-created which will persist the data across server restarts.
When you re-create the container, a new volume is created and you can't access the old data unless you manually mount the old volume.
See https://docs.docker.com/storage/volumes/ for more information.

To persist the game server data on the host filesystem, use `-v C:/path/on/host:C:/Users/ContainerUser/arma3server` when creating the docker container.

### Important note about permissions when using bind mounts
When you use bind mounts, you need to make sure that the `ContainerUser` inside the container has access to the files.
This should be the case by default, when the host directory has been created with a non-admin account.
Otherwise, an ACL can be added, which grants the group "Authenticated Users" modify, read and execute, list folder contents, read, and write access to the host directory.

## Ports
* 2302/udp (game)
* 2303/udp (query) - always game+1
* 2304/udp (steam) - always game+2
* 2305/udp (VON) - always game+3
* 2306/udp (battlEye) - always game+4

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

`STEAM_USERNAME` The username of a steam account owning ARMA III. Required for the managed mods feature.

`STEAM_PASSWORD` The password of a steam account owning ARMA III. Required for the managed mods feature.


### start
The following env variables are always available during `start`.

`MODE` Set to `client` to start a headless client instead of a server

`USE_X64` Set to `false` to disable the use of x64 binaries.

`PROFILE` The profile to use. Defaults to `server`.

`LIMIT_FPS` The maximum amount of server FPS. Numeric value between 5 and 1000.

### server mode
The following env variables are available during `start` when in `server` mode (the default mode).

`HOST` The host address, the server should listen on.

`GAME_PORT` The game port to use. Remember to also update the container port bindings when changing this variable.

`BASIC_CFG` The name of the basic config file.

`SERVER_CFG` The name of the server config file.

`AUTO_INIT` Set to `true` to automatically initialize the mission on server start.

`LOAD_MISSION_TO_MEMORY` Set to `true` to load the mission into memory once the first client downloads it. This requires more memory, but can save some CPU cycles.

### client mode
The following env variables are available during `start` when in `client` mode.

`GAME_SERVER_IP` The ip of the game server to connect to. This must be set.

`GAME_SERVER_PORT` The port of the game server to connect to. Defaults to 2302.

`GAME_SERVER_PASSWORD` The passowrd of the game server to connect to.

## Mods
All mods should be installed in the subdirectory `mods`.

Mods can be automatically downloaded and kept up to date from the Steam workshop. Therefore you need to provide a space seperated list of steam workshop ids in the `MANAGED_MODS` env variable during `update` or `update_validate`. This will download the mods to the `mods` subdirectory.
Note that this will only install the mods but do not load them.

The following environment variables can be used to actually load the mods:

`MODS` A list of mods to load, seperated by semicolons. Each entry should be a relative path to the folder containing the mod. e.g. `mods/@ace;mods/@CBA_A3`.

`SERVER_MODS` Same like `MODS`, but used for server-side only mods. Clients will not see these mods. Only works when in the `server` mode.

In addition, when in `server` mode, mod keys can be automatically extracted to the `keys` subdirectory. To enable this, set `EXTRACT_MOD_KEYS=true`.

### Important note about container isolation
When using the managed mods feature, you can't use Hyper-V container isolation, because we need to create a Junction from one mount point to another.
Instead, you have to use process isolation using the `--isolation=process` flag when starting the container.
Process isolation is the default on Windows Server but not on Windows Client operating systems.
More context: https://github.com/moby/moby/issues/37024, https://docs.microsoft.com/en-us/virtualization/windowscontainers/manage-containers/hyperv-container
