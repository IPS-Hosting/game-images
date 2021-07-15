# alt:V

## Basic usage
For advanced usage, refer to https://docs.docker.com
```shell
# Create the docker container
docker create -it --restart always \
  --name altv-server \
  -p 7788:7788/tcp \
  -p 7788:7788/udp \
  ipshosting/game-altv:v2
  
# Start the server
docker start altv-server

# Stop the server
docker stop altv-server

# Restart the server
docker restart altv-server

# View server logs
docker logs altv-server

# Attach to server console to write commands and see output in realtime (de-attach by pressing CTRL-P + CTRL-Q).
docker attach altv-server

# Remove the container
docker rm altv-server
```

## Commands
By default, when starting the container, it will be installed and updated, and the alt:V Server is started afterwards.
You can create a container with a different command to change this behaviour:
* **install_update** Only install and update the server. It won't be started and the container will exit after the alt:V server is installed and updated.
* **start** Only start the alt:V server without installing or updating.

## Data persistence
Game server data is kept in `/home/ips-hosting`.
By default a volume will be auto-created which will persist the game server data across server restarts.
When you re-create the container, a new volume is created and you can't access the old data unless you manually mount the old volume.
See https://docs.docker.com/storage/volumes/ for more information.

To persist the game server data on the host filesystem, use `-v /absolute-path/on/host:/home/ips-hosting` when creating the docker container.

# Ports
* 7788/tcp (http)
* 7788/udp (game)

You can change the port with the `PORT` environment variable.

## Env variables
Env variables can be configured with the `-e "KEY=VAL"` flag when creating the container. The flag can be used multiple times.
To change the env variables, you need to re-create the container.

`HOST` The host the server listens on. Defaults to `0.0.0.0`

`PORT` The port the server listens on. Defaults to `7788`.

## NPM / Yarn support
When installing and updating the server, node packages will be automatically installed using `yarn` or `npm` when there is a `yarn.lock` or `package-lock.json` file.
Also, when there is a`package.json` file with a `build` script, it will automatically be run.
