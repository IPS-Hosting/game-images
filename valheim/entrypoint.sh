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
}

function update_validate() {
	ensure_steamcmd
	cd /tmp/steamcmd

	if [ -n "${BETA_BRANCH}" ] && [ -n "${BETA_PASSWORD}" ]; then
		./steamcmd.sh +login anonymous +force_install_dir /home/ips-hosting +app_update 896660 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" validate +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +login anonymous +force_install_dir /home/ips-hosting +app_update 896660 -beta "${BETA_BRANCH}" validate +quit
	else
		./steamcmd.sh +login anonymous +force_install_dir /home/ips-hosting +app_update 896660 validate +quit
	fi
}

function update() {
	ensure_steamcmd
	cd /tmp/steamcmd

	if [ -n "${BETA_BRANCH}" ] && [ -n "${BETA_PASSWORD}" ]; then
		./steamcmd.sh +login anonymous +force_install_dir /home/ips-hosting +app_update 896660 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +login anonymous +force_install_dir /home/ips-hosting +app_update 896660 -beta "${BETA_BRANCH}" +quit
	else
		./steamcmd.sh +login anonymous +force_install_dir /home/ips-hosting +app_update 896660 +quit
	fi
}

function start() {
	local start_command="./valheim_server.x86_64 -port ${GAME_PORT:-2456} -name '${NAME:-A Valheim server}' -world '${WORLD:-Dedicated}' -password '${PASSWORD:-secret}' -savedir '/home/ips-hosting/data'"
	
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
