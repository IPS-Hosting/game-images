# Rust

GitHub: https://github.com/IPS-Hosting/game-images/tree/main/rust

## Basic usage
For advanced usage, refer to https://docs.docker.com
```shell
# Create the docker container
docker create -it --restart always \
  --name rust-server \
  -p 28015:28015/udp \
  -p 28015:28015/tcp \
  -p 28082:28082/tcp \
  ipshosting/game-rust:v2
  
# Start the server
docker start rust-server

# Stop the server
docker stop rust-server

# Restart the server
docker restart rust-server

# View server logs
docker logs rust-server

# Attach to server console to write commands and see output in realtime (de-attach by pressing CTRL-P + CTRL-Q).
docker attach rust-server

# Remove the container
docker rm rust-server
```

## Commands
By default, when starting the container, it will be installed and updated, and the rust Server is started afterwards.
You can create a container with a different command to change this behaviour:
* **update** Only install the latest version of the server. It won't be started and the container will exit after the rust server is installed and updated.
* **update_validate** Same like update but will also validate the files. Recommended for the initial installation of the server.
* **start** Only start the rust server without installing or updating.

## Data persistence
Game server data is kept in `/home/ips-hosting`.
By default a volume will be auto-created which will persist the game server data across server restarts.
When you re-create the container, a new volume is created and you can't access the old data unless you manually mount the old volume.
See https://docs.docker.com/storage/volumes/ for more information.

To persist the game server data on the host filesystem, use `-v /absolute-path/on/host:/home/ips-hosting` when creating the docker container.

## Ports
* 28015/udp (game)
* 28015/tcp (rcon)
* 28082/tcp (app)

You can change the ports with the `GAME_PORT`, `RCON_PORT` and `APP_PORT` environment variables.

## Env variables
Env variables can be configured with the `-e "KEY=VAL"` flag when creating the container. The flag can be used multiple times.
To change the env variables, you need to re-create the container.

### update and update_validate
The following env variables are available during `update` and `update_validate`.

`BETA_BRANCH` Used to download a different branch of the server.

`BETA_PASSWORD` The password for the beta branch.


### start
The following env variables are always available during `start`.

`HOST` The host address, the server listens on. Defaults to `0.0.0.0`

`GAME_PORT` The game port to use. Defaults to `28015`. Remember to also update the container port bindings when changing this variable.

`TICKRATE` The tickrate the server uses. Defaults to `30`.

`HOSTNAME` The hostname of the server.

`DESCRIPTION` The server description, showed in the server browser, before joining the server.

`URL` The URL of the server. A button that links to this URL is showed in the server browser, before joining the server.

`HEADER_IMAGE` A URL to an image file. Used as a header image in the server browser, before joining the server.

`LOGO_IMAGE` A URL to an image file. Used as the logo for the server in the server browser. See https://wiki.facepunch.com/rust/custom-server-icon.

`IDENTITY` The identity of the server. Config files and data are stored in a sub-folder with this name. Defaults to `default`.

`GAMEMODE` The gamemode the server should run. Defaults to `vanilla`. See https://wiki.facepunch.com/rust/server-gamemodes for a list of available gamemodes.

`MAX_PLAYERS` The maximum amount of players that can join the server. Defaults to `50`.

`SAVE_INTERVAL` Am integer value between `30` and `600`. The amount of seconds between each save. Defaults to `300`.

`RCON_PASSWORD` The RCON password. Required to enable RCON.

`RCON_WEB` Set to `1` to enable web based RCON. A value of `0` means legacy RCON. Defaults to `1`.

`RCON_PORT` The rcon port to use. Defaults to `28015`. Remember to also update the container port bindings when changing this variable.

`LEVEL` The level the server should run. Defaults to `Procedural Map`.

`WORLD_SIZE` The generated map size in meters. Must be an integer value between `1000` and `6000`. Defaults to `3000`.

`SEED` An integer value between `1` and `2147483647` which is used for procedural map generation. The same seed will generate the same map.

`SALT` An integer value between `1` and `2147483647` which is used to generate resource spawn points. The same salt will generate the same spawn points.

`LEVEL_URL` A URL to a custom map. See https://wiki.facepunch.com/rust/Hosting_a_custom_map.

`APP_PUBLIC_IP` The public ip of the server. Required to enable the Rust+ companion app.

`APP_PORT` The app port to use for Rust+. Defaults to `28082`. Remember to also update the container port bindings when changing this variable.

`CENTRALIZED_BANNING_ENDPOINT` A URL pointing to a web server which hosts the centralized banning API. See https://wiki.facepunch.com/rust/centralized-banning.

`CENTRALIZED_BANNING_FAILURE_MODE` See https://wiki.facepunch.com/rust/centralized-banning#whathappenswhentheapiendpointisdownornotworkingproperly. Defaults to `0`.

`CENTRALIZED_BANNING_TIMEOUT` The timeout for requests to the centralized banning endpoint in seconds. Defaults to `5`.

`TAGS` The tags for the server. See https://wiki.facepunch.com/rust/server-browser-tags.
