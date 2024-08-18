# Palworld

GitHub: https://github.com/IPS-Hosting/game-images/tree/main/palworld

Palworld Docs: https://tech.palworldgame.com/community-server-guide

## Basic usage

For advanced usage, refer to https://docs.docker.com

```shell
# Create the docker container
docker create -it --restart always \
  --name palworld-server \
  -p 8211:8211/udp \
  ipshosting/game-palworld:v1

# Start the server
docker start palworld-server

# Stop the server
docker stop palworld-server

# Restart the server
docker restart palworld-server

# View server logs
docker logs palworld-server

# Attach to server console to write commands and see output in realtime (de-attach by pressing CTRL-P + CTRL-Q).
docker attach palworld-server

# Remove the container
docker rm palworld-server
```

## Commands

By default, when starting the container, it will be installed and updated, and the palworld Server is started afterwards.
You can create a container with a different command to change this behaviour:

- **update** Only install the latest version of the server. It won't be started and the container will exit after the palworld server is installed and updated.
- **update_validate** Same like update but will also validate the files. Recommended for the initial installation of the server.
- **start** Only start the palworld server without installing or updating.

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

- 8211/udp (game)
- 25575/tcp (rcon)

You can change the game port with the `GAME_PORT` environment variable.
The RCON port can be changed using the `RCONPort` setting in the `PalWorldSettings.ini` file. Note that RCON is disabled by default and needs to be enabled in the same file using the `RCONEnabled` setting.

## Env variables

Env variables can be configured with the `-e "KEY=VAL"` flag when creating the container. The flag can be used multiple times.
To change the env variables, you need to re-create the container.

### update and update_validate

The following env variables are available during `update` and `update_validate`.

`BETA_BRANCH` Used to download a different branch of the server.

`BETA_PASSWORD` The password for the beta branch.

### start

The following env variables are always available during `start`.

`GAME_PORT` The game port to use. Defaults to `8211`. Remember to also update the container port bindings when changing this variable.

`MAX_PLAYERS` Maximum amount of players that can connect. Defaults to `32`.

`LOG_FORMAT` The format of the logs outputted by the server. Can be `text` or `json`. Defaults to `text`.

`ENABLE_MULTI_THREADING` Set to `true` to improve performance in multi-threaded CPU environments. Effective up to a maximum of 4 CPU cores.

`ENABLE_COMMUNITY_SERVER` Set to `true` to make the server appear in the list of community servers.

`PUBLIC_IP` You can manually specify the global IP address of the network on which the server is running.
If not specified, it will be detected automatically. If it does not work well, try manual configuration.

`PUBLIC_PORT` You can manually specify the port number of the network on which the server is running.
If not specified, it will be detected automatically. If it does not work well, try manual configuration.
