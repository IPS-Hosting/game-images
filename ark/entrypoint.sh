#!/bin/bash

set -o errexit
set -o pipefail

function apply_fixes() {
	# Required for -automanagedmods to work (https://steamcommunity.com/app/346110/discussions/0/2523653167135099429/)
	ensure_steamcmd_ark
	ln -svf ../../../../../Steam/steamapps /home/ips-hosting/Engine/Binaries/ThirdParty/SteamCMD/Linux/steamapps

	# Setup useful symlinks
	ln -svf ShooterGame/Content/Mods /home/ips-hosting/Mods
	ln -svf ShooterGame/Saved/Config/LinuxServer/GameUserSettings.ini /home/ips-hosting/GameUserSettings.ini
	ln -svf ShooterGame/Saved/Config/LinuxServer/Game.ini /home/ips-hosting/Game.ini
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

function ensure_steamcmd_ark() {
	mkdir -vp /home/ips-hosting/Engine/Binaries/ThirdParty/SteamCMD/Linux
	cd /home/ips-hosting/Engine/Binaries/ThirdParty/SteamCMD/Linux

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
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 376030 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" validate +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 376030 -beta "${BETA_BRANCH}" validate +quit
	else
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 376030 validate +quit
	fi

	apply_fixes
}

function update() {
	ensure_steamcmd
	cd /tmp/steamcmd

	if [ -n "${BETA_BRANCH}" ] && [ -n "${BETA_PASSWORD}" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 376030 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 376030 -beta "${BETA_BRANCH}" +quit
	else
		./steamcmd.sh +force_install_dir /home/ips-hosting +login anonymous +app_update 376030 +quit
	fi

	apply_fixes
}

function start() {
	local start_command="./ShooterGameServer ${MAP:-TheIsland}?listen?MultiHome=${HOST:-0.0.0.0}?Port=${GAME_PORT:-7777}?QueryPort=${QUERY_PORT:-27015}?RCONPort=${RCON_PORT:-27020}?RCONEnabled=${RCON_ENABLED:-True}?MaxPlayers=${MAX_PLAYERS:-10}"
	if [ -n "$SESSION_NAME" ]; then
		start_command="$start_command?SessionName=$SESSION_NAME"
	fi
	if [ "$RAW_SOCKETS" == "true" ]; then
		start_command="$start_command?bRawSockets"
	fi
	if [ -n "$MODS" ]; then
		start_command="$start_command?GameModIds=$MODS"
	fi
	if [ -n "$SERVER_PASSWORD" ]; then
		start_command="$start_command?ServerPassword=$SERVER_PASSWORD"
	fi
	if [ -n "$SERVER_ADMIN_PASSWORD" ]; then
		start_command="$start_command?ServerAdminPasword=$SERVER_ADMIN_PASSWORD"
	fi
	if [ "$ALLOW_FLYER_CARRY_PVE" == "true" ]; then
		start_command="$start_command?AllowFlyerCarryPvE=true"
	fi
	start_command="$start_command -server -automanagedmods"
	if [ -n "$PUBLIC_IP" ]; then
		start_command="$start_command -crossplay -PublicIPForEpic=${PUBLIC_IP}"
	fi
	if [ "$SERVER_GAME_LOG" == "true" ]; then
		start_command="$start_command -servergamelog"
	fi
	if [ "$SERVER_GAME_LOG_INCLUDE_TRIBE_LOGS" == "true" ]; then
		start_command="$start_command -servergamelogincludetribelogs"
	fi
	if [ "$FORCE_ALLOW_CAVE_FLYERS" == "true" ]; then
		start_command="$start_command -ForceAllowCaveFlyers"
	fi
	if [ "$NO_BATTLEYE" == "true" ]; then
		start_command="$start_command -NoBattlEye"
	fi

	cd /home/ips-hosting/ShooterGame/Binaries/Linux
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
