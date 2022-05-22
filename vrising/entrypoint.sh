#!/bin/bash

set -o errexit
set -o pipefail

function ensure_steamcmd() {
	mkdir -vp /tmp/steamcmd
	cd /tmp/steamcmd

	if [ ! -f "./steamcmd.sh" ]; then
		wget https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz
		tar -xvzf steamcmd_linux.tar.gz
		rm steamcmd_linux.tar.gz
	fi

	# Workaround for https://www.reddit.com/r/SteamCMD/comments/nv9oey/error_failed_to_install_app_xxx_disk_write_failure/
	mkdir -vp /home/ips-hosting/steamapps
}

function update_validate() {
	ensure_steamcmd
	cd /tmp/steamcmd

	if [ -n "$BETA_BRANCH" ] && [ -n "$BETA_PASSWORD" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 1829350 -beta "$BETA_BRANCH" -betapassword "$BETA_PASSWORD" validate +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 1829350 -beta "$BETA_BRANCH" validate +quit
	else
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 1829350 validate +quit
	fi
}

function update() {
	ensure_steamcmd
	cd /tmp/steamcmd

	if [ -n "${BETA_BRANCH}" ] && [ -n "${BETA_PASSWORD}" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 1829350 -beta "$BETA_BRANCH" -betapassword "$BETA_PASSWORD" +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 1829350 -beta "$BETA_BRANCH" +quit
	else
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 1829350 +quit
	fi
}


function start() {
	local start_command="xvfb-run -a wine VRisingServer.exe -persistentDataPath ./data"

	# Allow to overwrite ServerHostSettings.json via env variables
	if [ -n "$HOST" ]; then
		start_command="$start_command -address $HOST"
	fi
	if [ -n "$GAME_PORT" ]; then
		start_command="$start_command -gamePort $GAME_PORT"
	fi
	if [ -n "$QUERY_PORT" ]; then
		start_command="$start_command -queryPort $QUERY_PORT"
	fi
	if [ -n "$SAVE_NAME" ]; then
		start_command="$start_command -saveName '$SAVE_NAME'"
	fi
	if [ -n "$SERVER_NAME" ]; then
		start_command="$start_command -serverName '$SERVER_NAME'"
	fi
	if [ -n "$MAX_CONNECTED_USERS" ]; then
		start_command="$start_command -maxConnectedUsers $MAX_CONNECTED_USERS"
	fi
	if [ -n "$MAX_CONNECTED_ADMINS" ]; then
		start_command="$start_command -maxConnectedAdmins $MAX_CONNECTED_ADMINS"
	fi
	
	cd /home/ips-hosting
	echo "$start_command"
	eval "$start_command"
}

case "$1" in
update)
	update
	;;
update_validate)
	update_validate
	;;
start)
	start
	;;
*)
	update_validate
	start
	;;
esac
