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

function post_update() {
	# Fixes: [S_API FAIL] SteamAPI_Init() failed; unable to locate a running instance of Steam,or a local steamclient.so.
	if [ ! -f "/home/ips-hosting/.steam/sdk32/steamclient.so" ]; then
		mkdir -vp /home/ips-hosting/.steam/sdk32
		cp -v /tmp/steamcmd/linux32/steamclient.so /home/ips-hosting/.steam/sdk32/steamclient.so
	fi
	# Fixes: [S_API] SteamAPI_Init(): Sys_LoadModule failed to load: /home/ips-hosting/.steam/sdk64/steamclient.so
	if [ ! -f "/home/ips-hosting/.steam/sdk64/steamclient.so" ]; then
		mkdir -vp /home/ips-hosting/.steam/sdk64
		cp -v /tmp/steamcmd/linux64/steamclient.so /home/ips-hosting/.steam/sdk64/steamclient.so
	fi

	# Create useful symlinks
	ln -svf FactoryGame/Saved/Config/LinuxServer /home/ips-hosting/Config
	ln -svf FactoryGame/Saved/Logs /home/ips-hosting/Logs
	ln -svf .config/Epic/FactoryGame/Saved/SaveGames /home/ips-hosting/SaveGames
}

function update_validate() {
	ensure_steamcmd
	cd /tmp/steamcmd

	if [ -n "${BETA_BRANCH}" ] && [ -n "${BETA_PASSWORD}" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 1690800 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" validate +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 1690800 -beta "${BETA_BRANCH}" validate +quit
	else
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 1690800 validate +quit
	fi

	post_update
}

function update() {
	ensure_steamcmd
	cd /tmp/steamcmd

	if [ -n "${BETA_BRANCH}" ] && [ -n "${BETA_PASSWORD}" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 1690800 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 1690800 -beta "${BETA_BRANCH}" +quit
	else
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 1690800 +quit
	fi

	post_update
}


function start() {
	local start_command="./UE4Server-Linux-Shipping FactoryGame -ServerQueryPort=${QUERY_PORT:-15777} -BeaconPort=${BEACON_PORT:-15000} -Port=${PORT:-7777} -DisablePacketRouting"

	export UE4_PROJECT_ROOT="/home/ips-hosting"
	export LD_LIBRARY_PATH="/home/ips-hosting/linux64:$LD_LIBRARY_PATH"

	cd /home/ips-hosting/Engine/Binaries/Linux
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
