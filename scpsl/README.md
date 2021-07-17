# SCP: Secret Laboratory

## Basic usage
For advanced usage, refer to https://docs.docker.com
```shell
# Create the docker container
docker create -it --restart always \
  --name scpsl-server \
  -p 28015:28015/udp \
  -p 28015:28015/tcp \
  -p 28082:28082/tcp \
  ipshosting/game-scpsl:v2
  
# Start the server
docker start scpsl-server

# Stop the server
docker stop scpsl-server

# Restart the server
docker restart scpsl-server

# View server logs
docker logs scpsl-server

# Attach to server console to write commands and see output in realtime (de-attach by pressing CTRL-P + CTRL-Q).
docker attach scpsl-server

# Remove the container
docker rm scpsl-server
```

## Commands
By default, when starting the container, it will be installed and updated, and the scpsl Server is started afterwards.
You can create a container with a different command to change this behaviour:
* **update** Only install the latest version of the server. It won't be started and the container will exit after the scpsl server is installed and updated.
* **update_validate** Same like update but will also validate the files. Recommended for the initial installation of the server.
* **start** Only start the scpsl server without installing or updating.

## Data persistence
Game server data is kept in `/home/ips-hosting`.
By default a volume will be auto-created which will persist the game server data across server restarts.
When you re-create the container, a new volume is created and you can't access the old data unless you manually mount the old volume.
See https://docs.docker.com/storage/volumes/ for more information.

To persist the game server data on the host filesystem, use `-v /absolute-path/on/host:/home/ips-hosting` when creating the docker container.

## Ports
* 7777/udp (game)
* 7777/tcp (query) - always the same as game

You can change the port with the `GAME_PORT` environment variable.

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

`GAME_PORT` The game port to use. Defaults to `7777`. Remember to also update the container port bindings when changing this variable.

`LOG_LENGTH_LIMIT` Specifies the limit of characters in LocalAdmin log file. Suffixes k, M, G and T are supported. Defaults to `1G`.
	