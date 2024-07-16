# Satisfactory

GitHub: https://github.com/IPS-Hosting/game-images/tree/main/satisfactory
Wiki: https://satisfactory.fandom.com/wiki/Dedicated_servers

## Basic usage

For advanced usage, refer to https://docs.docker.com

```shell
# Create the docker container
docker create -it --restart always \
  --name satisfactory-server \
  -p 15777:15777/udp \
  -p 15000:15000/udp \
  -p 7777:7777/udp \
  ipshosting/game-satisfactory:v2

# Start the server
docker start satisfactory-server

# Stop the server
docker stop satisfactory-server

# Restart the server
docker restart satisfactory-server

# View server logs
docker logs satisfactory-server

# Attach to server console to write commands and see output in realtime (de-attach by pressing CTRL-P + CTRL-Q).
docker attach satisfactory-server

# Remove the container
docker rm satisfactory-server
```

## Commands

By default, when starting the container, it will be installed and updated, and the Satisfactory Server is started afterwards.
You can create a container with a different command to change this behaviour:

- **update** Only install the latest version of the server. It won't be started and the container will exit after the Satisfactory server is installed and updated.
- **update_validate** Same like update but will also validate the files. Recommended for the initial installation of the server.
- **start** Only start the Satisfactory server without installing or updating.

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

- 15777/udp (query)
- 15000/udp (beacon)
- 7777/udp (game)

You can change the ports with the `QUERY_PORT`, `BEACON_PORT` and `PORT` environment variables.

## Env variables

Env variables can be configured with the `-e "KEY=VAL"` flag when creating the container. The flag can be used multiple times.
To change the env variables, you need to re-create the container.

### update and update_validate

The following env variables are available during `update` and `update_validate`.

`BETA_BRANCH` Used to download a different branch of the server.

`BETA_PASSWORD` The password for the beta branch.
