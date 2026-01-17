# Hytale

Manual: https://support.hytale.com/hc/en-us/articles/45326769420827-Hytale-Server-Manual

## Authentication

You must own the Hytale game. When downloading server files, you will be asked to authenticate with your Hytale account.
Copy the authentication link to your browser and login with your Hytale account.
The oauth credentials are persistend on the server data volume at `.hytale-downloader-credentials.json`.
You can change the credential path using the `CREDENTIALS_PATH` env variable.

After the server is started (as indicated by the `Hytale Server Booted!` log line), run the following commands to authenticate your server:

```sh
# Complete the authentication flow by following the documented steps
/auth login device
# Optionally persist the authentication credentials
/auth persistence Encrypted
```

## Basic usage

For advanced usage, refer to https://docs.docker.com

```shell
# Create the docker container
docker create -it --restart always \
  --name hytale-server \
  TODO
  -p 7777:7777/tcp \
  -p 7777:7777/udp \
  -p 8888:8888/tcp \
  ipshosting/game-hytale:v3

# Start the server
docker start hytale-server

# Stop the server
docker stop hytale-server

# Restart the server
docker restart hytale-server

# View server logs
docker logs hytale-server

# Attach to server console to write commands and see output in realtime (de-attach by pressing CTRL-P + CTRL-Q).
docker attach hytale-server

# Remove the container
docker rm hytale-server
```

## Commands

By default, when starting the container, it will install/update the launcher, check for game updates, and start the Hytale Server.
You can create a container with a different command to change this behaviour:

- **install_update_launcher** Only install or update the Hytale downloader launcher. The server will not be started and the container will exit after the launcher is installed/updated.
- **install_update_game** Only download and install the game server. The server will not be started and the container will exit after the game is installed/updated.
- **start** Only start the Hytale server without installing or updating.

## Data persistence

Game server data is kept in `/home/ips-hosting`.
By default a volume will be auto-created which will persist the game server data across server restarts.
When you re-create the container, a new volume is created and you can't access the old data unless you manually mount the old volume.
See https://docs.docker.com/storage/volumes/ for more information.

To persist the game server data on the host filesystem, use `-v /absolute/path/on/host:/home/ips-hosting` when creating the docker container.
The container is run as a non-root user by default and the user running inside the container has the id 4711. Make sure that the mounted directory is readable and writable by the user running the container. There are 2 ways to achieve this:

- Change the owner of the host directory: `chown -R 4711 /absolute/path/on/host` OR
- Run the container as the user, which owns the files on the host system. Make sure to specify the id of your local user, because the name is uknown inside the container. You can find it out using `id YOUR_USERNAME`. Then run the docker command using the `--user USER_ID` flag. E.g.: `docker create --user 500 ...`.

## Ports

- 5520/udp (game)
- 5523/tcp (web server, provided by Nitrado WebServer plugin; defaults to game port +3)

You can change the game port using the `PORT` environment variable.
The WebServer plugin binds to `PORT + 3` on TCP by default. You can override this via `mods/Nitrado_WebServer/config.json`.
Remember to also update the container port bindings when changing port variables.

## Env variables

Env variables can be configured with the `-e "KEY=VAL"` flag when creating the container. The flag can be used multiple times.
To change the env variables, you need to re-create the container.

### install_update_launcher and install_update_game

The following env variables are available during `install_update_launcher` and `install_update_game`.

`CREDENTIALS_PATH` Path to the Hytale downloader credentials file. Defaults to `/home/ips-hosting/.hytale-downloader-credentials.json`.

`PATCHLINE` The patchline to download. Defaults to `release`.

`SKIP_UPDATE_CHECK` Set to `true` to skip checking for launcher and game updates. Defaults to `false`.

`FORCE_UPDATE` Set to `true` to force redownload of game files, even when already up to date. Defaults to `false`.

### start

The following env variables are always available during `start`.

`BIND` The address and port the server binds to. Defaults to `0.0.0.0:5520`.

`PORT` The port the server listens on. Defaults to `5520`. Remember to also update the container port bindings when changing this variable.

`ASSETS` Path to the assets directory. Defaults to `../HytaleAssets`.

`JVM_ARGS` Additional Java Virtual Machine arguments.

`AUTH_MODE` Authentication mode for the server. Options: `authenticated`, `offline`, `insecure`. Defaults to `authenticated`.

`TRANSPORT` Transport type. Defaults to `QUIC`.

`BACKUP` Set to `true` to enable automatic backups.

`BACKUP_DIR` Directory for backups.

`BACKUP_FREQUENCY` Frequency of backups in minutes. Defaults to `30`.

`BACKUP_MAX_COUNT` Maximum number of backups to keep. Defaults to `5`.

`OWNER_NAME` Name of the server owner.

`OWNER_UUID` UUID of the server owner.

`SESSION_TOKEN` Session token for Session Service API.

`IDENTITY_TOKEN` Identity token (JWT).

`UNIVERSE` Path to the universe data.

`WORLD_GEN` World gen directory.

`PREFAB_CACHE` Prefab cache directory for immutable assets.

`MODS` Additional mods directories to load from.

`EARLY_PLUGINS` Additional early plugin directories to load from.

`INSTALL_DEFAULT_PLUGINS` Automatically install common plugins into `mods/` before starting. Defaults to `true`. When enabled, the latest release JARs of the plugins listed in `DEFAULT_PLUGINS` will be downloaded from GitHub and placed into `/home/ips-hosting/mods`.

`DEFAULT_PLUGINS` Comma-separated list of default plugins to install when `INSTALL_DEFAULT_PLUGINS=true`. Defaults to `webserver,query,performance-saver,prometheus`. Supported values:

- `webserver` → nitrado/hytale-plugin-webserver
- `query` → nitrado/hytale-plugin-query
- `performance-saver` → nitrado/hytale-plugin-performance-saver
- `prometheus` → apexhosting/hytale-plugin-prometheus

`BOOT_COMMAND` Command to execute on boot. Multiple commands can be provided and will be executed synchronously in order.

`MIGRATE_WORLDS` Worlds to migrate.

`MIGRATIONS` The migrations to run.

`LOG` Sets the logger level.

`CLIENT_PID` Client process ID.

`FORCE_NETWORK_FLUSH` Force network flush. Defaults to `true`.

`VALIDATE_ASSETS` Set to `true` to validate assets on startup. The server will exit with an error code if any assets are invalid.

`VALIDATE_PREFABS` Validate prefabs on startup. The server will exit with an error code if any prefabs are invalid.

`VALIDATE_WORLD_GEN` Set to `true` to validate world generation on startup. The server will exit with an error code if default world gen is invalid.

`ACCEPT_EARLY_PLUGINS` Set to `true` to acknowledge that loading early plugins is unsupported and may cause stability issues.

`ALLOW_OP` Set to `true` to allow operator commands.

`BARE` Set to `true` to run the server bare (without loading worlds, binding to ports or creating directories). Note: Plugins will still be loaded which may not respect this flag.

`DISABLE_ASSET_COMPARE` Set to `true` to disable asset comparison.

`DISABLE_CPB_BUILD` Set to `true` to disable building of compact prefab buffers.

`DISABLE_FILE_WATCHER` Set to `true` to disable file watcher.

`DISABLE_SENTRY` Set to `true` to disable Sentry error reporting.

`EVENT_DEBUG` Set to `true` to enable event debugging.

`GENERATE_SCHEMA` Set to `true` to generate schema, save it into the assets directory and then exit.

`SHUTDOWN_AFTER_VALIDATE` Set to `true` to automatically shutdown the server after asset and/or prefab validation.

`SINGLEPLAYER` Set to `true` to run in singleplayer mode.
