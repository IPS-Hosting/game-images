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

function apply_fixes() {
	ensure_steamcmd
	# Fixes: [S_API FAIL] SteamAPI_Init() failed; unable to locate a running instance of Steam,or a local steamclient.so.
	if [ ! -f "/home/ips-hosting/.steam/sdk32/steamclient.so" ]; then
		mkdir -vp /home/ips-hosting/.steam/sdk32
		cp -v /tmp/steamcmd/linux32/steamclient.so /home/ips-hosting/.steam/sdk32/steamclient.so
	fi
}

function update_validate() {
	ensure_steamcmd
	cd /tmp/steamcmd

	if [ -n "${BETA_BRANCH}" ] && [ -n "${BETA_PASSWORD}" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 258550 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" validate +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 258550 -beta "${BETA_BRANCH}" validate +quit
	else
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 258550 validate +quit
	fi

	apply_fixes
}

function update() {
	ensure_steamcmd
	cd /tmp/steamcmd

	if [ -n "${BETA_BRANCH}" ] && [ -n "${BETA_PASSWORD}" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 258550 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 258550 -beta "${BETA_BRANCH}" +quit
	else
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 258550 +quit
	fi

	apply_fixes
}

function start() {
	local start_command="./RustDedicated -batchmode +server.ip ${HOST:-0.0.0.0} +server.port ${GAME_PORT:-28015} +server.tickrate ${TICKRATE:-30} +server.hostname '${HOSTNAME}' +server.description '${DESCRIPTION}' +server.url '${URL}' +server.headerimage '${HEADER_IMAGE}' +server.identity '${IDENTITY:-default}' +server.gamemode ${GAMEMODE:-vanilla} +server.level '${LEVEL:-Procedural Map}' +server.seed ${SEED} +server.salt ${SALT} +server.worldsize ${WORLD_SIZE:-3000} +server.maxplayers ${MAX_PLAYERS:-50} +server.saveinterval ${SAVE_INTERVAL:-300} +rcon.web ${RCON_WEB:-1} +rcon.ip ${HOST:-0.0.0.0} +rcon.port ${RCON_PORT:-28015} +rcon.password '${RCON_PASSWORD}' +app.listenip ${HOST:-0.0.0.0} +app.port ${APP_PORT:-28082} +app.publicip ${PUBLIC_IP} -logfile"
	
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
