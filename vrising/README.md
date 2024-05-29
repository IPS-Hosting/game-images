# V Rising

GitHub: https://github.com/IPS-Hosting/game-images/tree/main/vrising

Official Server Instructions: https://github.com/StunlockStudios/vrising-dedicated-server-instructions

## Basic usage
For advanced usage, refer to https://docs.docker.com
```shell
# Create the docker container
docker create -it --restart always \
  --name vrising-server \
  -p 27015:27015/udp \
  -p 27016:27016/udp \
  ipshosting/game-vrising:v1
  
# Start the server
docker start vrising-server

# Stop the server
docker stop vrising-server

# Restart the server
docker restart vrising-server

# View server logs
docker logs vrising-server

# Attach to server console to write commands and see output in realtime (de-attach by pressing CTRL-P + CTRL-Q).
docker attach vrising-server

# Remove the container
docker rm vrising-server
```

## Commands
By default, when starting the container, it will be installed and updated, and the vrising Server is started afterwards.
You can create a container with a different command to change this behaviour:
* **update** Only install the latest version of the server. It won't be started and the container will exit after the vrising server is installed and updated.
* **update_validate** Same like update but will also validate the files. Recommended for the initial installation of the server.
* **start** Only start the vrising server without installing or updating.

## Data persistence
Game server data is kept in `/home/ips-hosting`.
By default a volume will be auto-created which will persist the game server data across server restarts.
When you re-create the container, a new volume is created and you can't access the old data unless you manually mount the old volume.
See https://docs.docker.com/storage/volumes/ for more information.

To persist the game server data on the host filesystem, use `-v /absolute-path/on/host:/home/ips-hosting` when creating the docker container.

## Ports
* 27015/udp (game)
* 27016/udp (query)

You can change the ports in the `ServerHostSettings.json` file, or with the `GAME_PORT` and `QUERY_PORT` environment variables.

## Env variables
Env variables can be configured with the `-e "KEY=VAL"` flag when creating the container. The flag can be used multiple times.
To change the env variables, you need to re-create the container.

### update and update_validate
The following env variables are available during `update` and `update_validate`.

`BETA_BRANCH` Used to download a different branch of the server.

`BETA_PASSWORD` The password for the beta branch.


### start
The following env variables are always available during `start` to overwrite the values in `ServerHostSettings.json`.

`HOST` The host address, the server listens on.

`GAME_PORT` The game port to use. This port needs to be used when using the direct connect feature in game. Remember to also update the container port bindings when changing this variable.

`QUERY_PORT` The query port to use. Used for Steam Server List features. Remember to also update the container port bindings when changing this variable.

`SAVE_NAME` The name of the save file / directory.

`SERVER_NAME` The name of the server.

`MAX_USERS` The maximum amount of concurrent players on the server.

`MAX_ADMINS` The maximum amount of admins to allow connect even when server is full.
