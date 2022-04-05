# Project: Zomboid

GitHub: https://github.com/IPS-Hosting/game-images/tree/main/pz

## Basic usage
For advanced usage, refer to https://docs.docker.com
```shell
# Create the docker container
docker create -it --restart always \
  --name pz-server \
  -p 8766:8766/udp \
  -p 16261:16261/udp \
  ipshosting/game-pz:v1
  
# Start the server
docker start pz-server

# Stop the server
docker stop pz-server

# Restart the server
docker restart pz-server

# View server logs
docker logs pz-server

# Attach to server console to write commands and see output in realtime (de-attach by pressing CTRL-P + CTRL-Q).
docker attach pz-server

# Remove the container
docker rm pz-server
```

## Commands
By default, when starting the container, it will be installed and updated, and the pz Server is started afterwards.
You can create a container with a different command to change this behaviour:
* **update** Only install the latest version of the server. It won't be started and the container will exit after the pz server is installed and updated.
* **update_validate** Same like update but will also validate the files. Recommended for the initial installation of the server.
* **start** Only start the pz server without installing or updating.

## Data persistence
Game server data is kept in `/home/ips-hosting`.
By default a volume will be auto-created which will persist the game server data across server restarts.
When you re-create the container, a new volume is created and you can't access the old data unless you manually mount the old volume.
See https://docs.docker.com/storage/volumes/ for more information.

To persist the game server data on the host filesystem, use `-v /absolute-path/on/host:/home/ips-hosting` when creating the docker container.

## Ports
* 8766/udp (game)
* 16261/tcp (query)

You can change the ports with the `GAME_PORT` and `QUERY_PORT` environment variables.

## Env variables
Env variables can be configured with the `-e "KEY=VAL"` flag when creating the container. The flag can be used multiple times.
To change the env variables, you need to re-create the container.

### update and update_validate
The following env variables are available during `update` and `update_validate`.

`BETA_BRANCH` Used to download a different branch of the server.

`BETA_PASSWORD` The password for the beta branch.

### start
The following env variables are always available during `start`.

`MEMORY` The maximum amount of memory that is allocated to the JVM heap. Defaults to `2048m` (2GB).

`HOST` The host address, the server listens on. Defaults to `0.0.0.0`

`GAME_PORT` The game port to use. Defaults to `8766`. Remember to also update the container port bindings when changing this variable.

`QUERY_PORT` The query port to use. Defaults to `16261`. Remember to also update the container port bindings when changing this variable.

`SERVER_NAME` The internal name of the server. The server database, saves and configuration files use this name, so changing this will result in a new world being created, and you need to reconfigure your server in the new configuration files. Defaults to `servertest`.

`ADMIN_USERNAME` The username of the admin user. Defaults to `admin`.

`ADMIN_PASSWORD` The password of the admin user. Defaults to `#Change_Me!`.

`NO_STEAM` Set to `true` to disable Steam integration.

`STEAM_VAC` Whether Valve Anti Cheat is enabled. Valid values are `true` or `false`. Defaults to `true`.

`DEBUG` Set to `true` to enabled debug mode.

## Additional information
Additional server configuration can be done in the `Zomboid/Server/{SERVER_NAME}*.ini` files after the server is installed.
See https://pzwiki.net/wiki/Dedicated_Server for more information, including a list of server commands.
