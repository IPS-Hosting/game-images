#!/bin/bash

set -o errexit
set -o pipefail

function setup_wine() {
    if [ ! -d "$WINEPREFIX" ]; then
        wineboot
    fi

    # Install .NET Desktop Runtime 9
    if [ ! -f "$WINEPREFIX/drive_c/Program Files/dotnet" ]; then
        mkdir -vp /tmp/dotnet
        cd /tmp/dotnet
        wget https://download.visualstudio.microsoft.com/download/pr/685792b6-4827-4dca-a971-bce5d7905170/1bf61b02151bc56e763dc711e45f0e1e/windowsdesktop-runtime-9.0.0-win-x64.exe
        xvfb-run -a wine ./windowsdesktop-runtime-9.0.0-win-x64.exe /quiet /norestart
    fi
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
    setup_wine
	ensure_steamcmd
	cd /tmp/steamcmd

	if [ -n "${BETA_BRANCH}" ] && [ -n "${BETA_PASSWORD}" ]; then
		./steamcmd.sh +@sSteamCmdForcePlatformType windows +force_install_dir /home/ips-hosting +login anonymous +app_update 1892930 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" validate +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +@sSteamCmdForcePlatformType windows +force_install_dir /home/ips-hosting +login anonymous +app_update 1892930 -beta "${BETA_BRANCH}" validate +quit
	else
		./steamcmd.sh +@sSteamCmdForcePlatformType windows +force_install_dir /home/ips-hosting +login anonymous +app_update 1892930 validate +quit
	fi
}

function update() {
	ensure_steamcmd
	cd /tmp/steamcmd

	if [ -n "${BETA_BRANCH}" ] && [ -n "${BETA_PASSWORD}" ]; then
		./steamcmd.sh +@sSteamCmdForcePlatformType windows +force_install_dir /home/ips-hosting +login anonymous +app_update 1892930 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +@sSteamCmdForcePlatformType windows +force_install_dir /home/ips-hosting +login anonymous +app_update 1892930 -beta "${BETA_BRANCH}" +quit
	else
		./steamcmd.sh +@sSteamCmdForcePlatformType windows +force_install_dir /home/ips-hosting +login anonymous +app_update 1892930 +quit
	fi
}

function start() {
	local start_command="xvfb-run -a wine ./sbox-server.exe +game ${SBOX_GAME:-facepunch.walker facepunch.flatgrass}"
	if [ -n "$SBOX_HOSTNAME" ]; then
		start_command="$start_command +hostname '$SBOX_HOSTNAME'"
	fi
	if [ -n "$SBOX_STEAM_TOKEN" ]; then
		start_command="$start_command +net_game_server_token '$SBOX_STEAM_TOKEN'"
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
