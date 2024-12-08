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

	if [ -n "${BETA_BRANCH}" ] && [ -n "${BETA_PASSWORD}" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 1892930 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" validate +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 1892930 -beta "${BETA_BRANCH}" validate +quit
	else
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 1892930 validate +quit
	fi
}

function update() {
	ensure_steamcmd
	cd /tmp/steamcmd

	if [ -n "${BETA_BRANCH}" ] && [ -n "${BETA_PASSWORD}" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 1892930 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 1892930 -beta "${BETA_BRANCH}" +quit
	else
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 1892930 +quit
	fi
}

function start() {
	# TODO
	# Replace -Xmx value in ProjectZomboid64.json
	sed -i "s/-Xmx.*/-Xmx${MEMORY:-2048m}\",/g" /home/ips-hosting/ProjectZomboid64.json

	# Workaround for 0.0.0.0 not working
	if [ "${HOST:-0.0.0.0}" == "0.0.0.0" ]; then
		HOST="$(hostname -I)"
	fi

	local start_command="./start-server.sh -ip ${HOST} -port ${QUERY_PORT:-16261} -steamport1 ${GAME_PORT:-8766} -cachedir=/home/ips-hosting/Zomboid -servername '${SERVER_NAME:-servertest}' -adminusername '${ADMIN_USERNAME:-admin}' -adminpassword '${ADMIN_PASSWORD:-#Change_Me!}' -steamvac ${STEAM_VAC:-true}"
	if [ "$NO_STEAM" == "true" ]; then
		start_command="$start_command -nosteam"
	fi
	if [ "$DEBUG" == "true" ]; then
		start_command="$start_command -debug"
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
