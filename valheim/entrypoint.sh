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
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 896660 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" validate +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 896660 -beta "${BETA_BRANCH}" validate +quit
	else
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 896660 validate +quit
	fi
}

function update() {
	ensure_steamcmd
	cd /tmp/steamcmd

	if [ -n "${BETA_BRANCH}" ] && [ -n "${BETA_PASSWORD}" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 896660 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 896660 -beta "${BETA_BRANCH}" +quit
	else
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 896660 +quit
	fi
}

function start() {
	# Set ENV variables required by ValheimPlus, if it is installed.
	# https://github.com/valheimPlus/ValheimPlus/blob/799a52a225487cc52e01a3cd77b37aa0e50be048/resources/unix/start_server_bepinex.sh
    if [ -d "/home/ips-hosting/BepInEx" ]; then
		export DOORSTOP_ENABLE=TRUE
		export DOORSTOP_INVOKE_DLL_PATH=/home/ips-hosting/BepInEx/core/BepInEx.Preloader.dll
		export DOORSTOP_CORLIB_OVERRIDE_PATH=/home/ips-hosting//unstripped_corlib
		export LD_LIBRARY_PATH=/home/ips-hosting/doorstop_libs:$LD_LIBRARY_PATH
		export LD_PRELOAD=libdoorstop_x64.so:$LD_PRELOAD
		export DYLD_LIBRARY_PATH=/home/ips-hosting/doorstop_libs
		export DYLD_INSERT_LIBRARIES=/home/ips-hosting/doorstop_libs/libdoorstop_x64.so
	fi

    # Set ENV variables required by Valheim server.
	export LD_LIBRARY_PATH=./linux64:$LD_LIBRARY_PATH
	export SteamAppId=892970

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
