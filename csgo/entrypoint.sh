#!/bin/bash

set -o errexit
set -o pipefail

function apply_fixes() {
	# Fixes: [S_API FAIL] SteamAPI_Init() failed; unable to locate a running instance of Steam,or a local steamclient.so.
	ensure_steamcmd
	if [ ! -f "/home/ips-hosting/.steam/sdk32/steamclient.so" ]; then
		mkdir -vp /home/ips-hosting/.steam/sdk32
		cp -v /ips-hosting/steamcmd/linux32/steamclient.so /home/ips-hosting/.steam/sdk32/steamclient.so
	fi

	# Fixes: Error parsing BotProfile.db - unknown attribute 'Rank'".
	# Comments out the Rank attribute.
	sed -i 's/^\s*Rank/\t\/\/ Rank/g' /home/ips-hosting/csgo/botprofile.db

	# Fixes: Unknown command "cl_bobamt_vert" and more.
	# Comments out exec default.cfg which includes the unknown commands.
	sed -i 's/^\s*exec\s*default.cfg/\/\/ exec default.cfg/g' /home/ips-hosting/csgo/cfg/valve.rc

	# Fixes exec: couldn't exec joystick.cfg.
	# Comments out exec joystick.cfg which doesn't exist.
	sed -i 's/^\s*exec\s*joystick.cfg/\/\/ exec joystick.cfg/g' /home/ips-hosting/csgo/cfg/valve.rc
}

function ensure_steamcmd() {
	mkdir -vp /ips-hosting/steamcmd
	cd /ips-hosting/steamcmd

	if [ ! -f "./steamcmd.sh" ]; then
		wget https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz
		tar -xvzf steamcmd_linux.tar.gz
		rm steamcmd_linux.tar.gz
	fi
}

function update_validate() {
	ensure_steamcmd
	cd /ips-hosting/steamcmd

	if [ -n "${BETA_BRANCH}" ] && [ -n "${BETA_PASSWORD}" ]; then
		./steamcmd.sh +login "$STEAM_USERNAME" "$STEAM_PASSWORD" +force_install_dir /home/ips-hosting +app_update 740 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" validate +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +login "$STEAM_USERNAME" "$STEAM_PASSWORD" +force_install_dir /home/ips-hosting +app_update 740 -beta "${BETA_BRANCH}" validate +quit
	else
		./steamcmd.sh +login "$STEAM_USERNAME" "$STEAM_PASSWORD" +force_install_dir /home/ips-hosting +app_update 740 validate +quit
	fi

	apply_fixes
}

function update() {
	ensure_steamcmd
	cd /ips-hosting/steamcmd

	if [ -n "${BETA_BRANCH}" ] && [ -n "${BETA_PASSWORD}" ]; then
		./steamcmd.sh +login "$STEAM_USERNAME" "$STEAM_PASSWORD" +force_install_dir /home/ips-hosting +app_update 740 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +login "$STEAM_USERNAME" "$STEAM_PASSWORD" +force_install_dir /home/ips-hosting +app_update 740 -beta "${BETA_BRANCH}" +quit
	else
		./steamcmd.sh +login "$STEAM_USERNAME" "$STEAM_PASSWORD" +force_install_dir /home/ips-hosting +app_update 740 +quit
	fi

	apply_fixes
}

function start() {
	local start_command="./srcds_run -game csgo -ip ${HOST:-0.0.0.0} -port ${GAME_PORT:-27015} -clientport ${CLIENT_PORT:-27005} +tv_port ${TV_PORT:-27020} -strictportbind -console -usercon -maxplayers_override ${MAX_PLAYERS:-10} -tickrate ${TICKRATE:-66} +game_type ${GAME_TYPE:-0} +game_mode ${GAME_MODE:-0} +mapgroup ${MAP:-mg_active} +map ${MAP:-de_mirage} +workshop_start_map ${WORKSHOP_START_MAP} +host_workshop_collection ${HOST_WORKSHOP_COLLECTION} -authkey ${AUTHKEY} +sv_setsteamaccount=$GSLT -nobreakpad"
	if [ "$INSECURE" == "true" ]; then
		start_command="$start_command -insecure"
	fi
	if [ "$NOBOTS" == "true" ]; then
		start_command="$start_command -nobots"
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
