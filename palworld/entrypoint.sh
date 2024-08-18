#!/bin/bash

set -o errexit
set -o pipefail

function post_update() {
	# Fixes: [S_API] SteamAPI_Init(): Sys_LoadModule failed to load: /home/ips-hosting/.steam/sdk64/steamclient.so
	if [ ! -f "/home/ips-hosting/.steam/sdk64/steamclient.so" ]; then
		mkdir -vp /home/ips-hosting/.steam/sdk64
		cp -v /tmp/steamcmd/linux64/steamclient.so /home/ips-hosting/.steam/sdk64/steamclient.so
	fi

	# Setup useful symlinks
	ln -svf Pal/Saved/Config/LinuxServer/PalWorldSettings.ini /home/ips-hosting/PalWorldSettings.ini
}

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
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 2394010 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" validate +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 2394010 -beta "${BETA_BRANCH}" validate +quit
	else
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 2394010 validate +quit
	fi

	post_update
}

function update() {
	ensure_steamcmd
	cd /tmp/steamcmd

	if [ -n "${BETA_BRANCH}" ] && [ -n "${BETA_PASSWORD}" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 2394010 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 2394010 -beta "${BETA_BRANCH}" +quit
	else
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 2394010 +quit
	fi

	post_update
}

function init_config() {
	# Copy default config if no custom config exists
    if [ ! -f "/home/ips-hosting/Pal/Saved/Config/LinuxServer/PalWorldSettings.ini" ]; then
        if [ ! -d "/home/ips-hosting/Pal/Saved/Config/LinuxServer" ]; then
            mkdir -p /home/ips-hosting/Pal/Saved/Config/LinuxServer
        fi

        cp -v /home/ips-hosting/DefaultPalWorldSettings.ini /home/ips-hosting/Pal/Saved/Config/LinuxServer/PalWorldSettings.ini
    fi
}

function start() {
	init_config

	local start_command="./Pal/Binaries/Linux/PalServer-Linux-Shipping Pal -port ${GAME_PORT:-8211} -players ${MAX_PLAYERS:-32} -logformat ${LOG_FORMAT:-text}"

	if [ -n "$PUBLIC_IP" ]; then
		start_command="$start_command -publicip ${PUBLIC_IP}"
	fi

	if [ -n "$PUBLIC_PORT" ]; then
		start_command="$start_command -publicport ${PUBLIC_PORT}"
	fi

	if [ "$ENABLE_COMMUNITY_SERVER" == "true" ]; then
		start_command="$start_command -publiclobby"
	fi

	if [ "$ENABLE_MULTI_THREADING" == "true" ]; then
		start_command="$start_command -useperfthreads -NoAsyncLoadingThread -UseMultithreadForDS"
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
