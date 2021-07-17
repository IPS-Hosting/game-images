# Valheim

## Basic usage
For advanced usage, refer to https://docs.docker.com
```shell
# Create the docker container
docker create -it --restart always \
  --name valheim-server \
  -p 2456:2456/udp \
  -p 2457:2457/udp \
  ipshosting/game-valheim:v2
  
# Start the server
docker start valheim-server

# Stop the server
docker stop valheim-server

# Restart the server
docker restart valheim-server

# View server logs
docker logs valheim-server

# Attach to server console to write commands and see output in realtime (de-attach by pressing CTRL-P + CTRL-Q).
docker attach valheim-server

# Remove the container
docker rm valheim-server
```

## Commands
By default, when starting the container, it will be installed and updated, and the valheim Server is started afterwards.
You can create a container with a different command to change this behaviour:
* **update** Only install the latest version of the server. It won't be started and the container will exit after the valheim server is installed and updated.
* **update_validate** Same like update but will also validate the files. Recommended for the initial installation of the server.
* **start** Only start the valheim server without installing or updating.

## Data persistence
Game server data is kept in `/home/ips-hosting`.
By default a volume will be auto-created which will persist the game server data across server restarts.
When you re-create the container, a new volume is created and you can't access the old data unless you manually mount the old volume.
See https://docs.docker.com/storage/volumes/ for more information.

To persist the game server data on the host filesystem, use `-v /absolute-path/on/host:/home/ips-hosting` when creating the docker container.

## Ports
* 2456/udp (game)
* 2457/udp (query) - always game+1

You can change the port with the `GAME_PORT` environment variable.

## Env variables
Env variables can be configured with the `-e "KEY=VAL"` flag when creating the container. The flag can be used multiple times.
To change the env variables, you need to re-create the container.

### update and update_validate
The following env variables are available during `update` and `update_validate`.

`BETA_BRANCH` Used to download a different branch of the server.

`BETA_PASSWORD` The password for the beta branch.


### start
The following env variables are always available during `start`.

`GAME_PORT` The game port to use. Defaults to `2456`. Remember to also update the container port bindings when changing this variable.

`NAME` The name of the server. Defaults to `A Valheim server`

`WORLD` The file name of the world. Defaults to `Dedicated`.

`PASSWORD` The password of the server. Can't be empty and needs to be at least 5 charactgers long. Must not be contained in the server name. Defaults to `secret`.
