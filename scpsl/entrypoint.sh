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
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 996560 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" validate +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 996560 -beta "${BETA_BRANCH}" validate +quit
	else
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 996560 validate +quit
	fi
}

function update() {
	ensure_steamcmd
	cd /tmp/steamcmd

	if [ -n "${BETA_BRANCH}" ] && [ -n "${BETA_PASSWORD}" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 996560 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 996560 -beta "${BETA_BRANCH}" +quit
	else
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 996560 +quit
	fi
}

function start() {
	cd /home/ips-hosting

	# Setup useful symlinks
	ln -svf ".config/SCP Secret Laboratory/ServerLogs/${GAME_PORT:-7777}" /home/ips-hosting/ServerLogs
	ln -svf ".config/SCP Secret Laboratory/LocalAdminLogs/${GAME_PORT:-7777}" /home/ips-hosting/LocalAdminLogs

	# Ensure .config folder exists: https://github.com/northwood-studios/LocalAdmin-V2/issues/52
	mkdir -vp .config

	local start_command="./LocalAdmin ${GAME_PORT:-7777} --printStd --noSetCursor --config /home/ips-hosting/localadmin_config.txt --useDefault --logLengthLimit ${LOG_LENGTH_LIMIT:-1G} --gameLogs '/home/ips-hosting/.config/SCP Secret Laboratory/ServerLogs/${GAME_PORT:-7777}' --logs '/home/ips-hosting/.config/SCP Secret Laboratory/LocalAdminLogs/${GAME_PORT:-7777}'"
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
