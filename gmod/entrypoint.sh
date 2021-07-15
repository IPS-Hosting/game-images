#!/bin/bash

set -o errexit
set -o pipefail

function download_content {
	mkdir -vp /ips-hosting/tmp/game-content/css
	ensure_steamcmd
	cd /ips-hosting/steamcmd
	./steamcmd.sh +login anonymous +force_install_dir /ips-hosting/tmp/game-content/css +app_update 232330 validate +quit

	mkdir -vp /home/ips-hosting/.ips-hosting/content/cstrike

	rsync \
		--verbose \
		--recursive \
		--human-readable \
		--progress \
		--stats \
		--delete \
		--prune-empty-dirs \
		--include "*/" \
		--include "*.vpk*" \
		--exclude "*" \
		/ips-hosting/tmp/game-content/css/cstrike/ \
		/home/ips-hosting/.ips-hosting/content/cstrike

	rm -rf /ips-hosting/tmp
}

function mount_content() {
	cp -vf /ips-hosting/mount.cfg /home/ips-hosting/garrysmod/cfg/mount.cfg
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

function apply_fixes() {
	ensure_steamcmd
	# Fixes: [S_API FAIL] SteamAPI_Init() failed; unable to locate a running instance of Steam,or a local steamclient.so.
	if [ ! -f "/home/ips-hosting/.steam/sdk32/steamclient.so" ]; then
		mkdir -vp /home/ips-hosting/.steam/sdk32
		cp -v /ips-hosting/steamcmd/linux32/steamclient.so /home/ips-hosting/.steam/sdk32/steamclient.so
	fi
}

function update_validate() {
	ensure_steamcmd
	cd /ips-hosting/steamcmd

	if [ -n "${BETA_BRANCH}" ] && [ -n "${BETA_PASSWORD}" ]; then
		./steamcmd.sh +login "$STEAM_USERNAME" "$STEAM_PASSWORD" +force_install_dir /home/ips-hosting +app_update 4020 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" validate +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +login "$STEAM_USERNAME" "$STEAM_PASSWORD" +force_install_dir /home/ips-hosting +app_update 4020 -beta "${BETA_BRANCH}" validate +quit
	else
		./steamcmd.sh +login "$STEAM_USERNAME" "$STEAM_PASSWORD" +force_install_dir /home/ips-hosting +app_update 4020 validate +quit
	fi

	apply_fixes
	download_content
	mount_content
}

function update() {
	ensure_steamcmd
	cd /ips-hosting/steamcmd

	if [ -n "${BETA_BRANCH}" ] && [ -n "${BETA_PASSWORD}" ]; then
		./steamcmd.sh +login "$STEAM_USERNAME" "$STEAM_PASSWORD" +force_install_dir /home/ips-hosting +app_update 4020 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +login "$STEAM_USERNAME" "$STEAM_PASSWORD" +force_install_dir /home/ips-hosting +app_update 4020 -beta "${BETA_BRANCH}" +quit
	else
		./steamcmd.sh +login "$STEAM_USERNAME" "$STEAM_PASSWORD" +force_install_dir /home/ips-hosting +app_update 4020 +quit
	fi

	apply_fixes
}


function start() {
	local start_command=""
	if [ "$USE_X64" == "true" ]; then
		start_command="./srcds_run_x64"
	else
		start_command="./srcds_run"
	fi
	start_command="$start_command -game garrysmod -ip ${HOST:-0.0.0.0} -port ${GAME_PORT:-27015} -clientport ${CLIENT_PORT:-27015} -strictportbind +maxplayers ${MAX_PLAYERS:-10} -tickrate ${TICKRATE:-66} +map ${MAP:-gm_construct} +gamemode ${GAMEMODE:-sandbox} +host_workshop_collection ${HOST_WORKSHOP_COLLECTION} +sv_setsteamaccount ${GSLT} -nohltv -norestart"
	if [ "$INSECURE" == "true" ]; then
		start_command="$start_command -insecure"
	fi
	if [ "$NOBOTS" == "true" ]; then
		start_command="$start_command -nobots"
	fi
	if [ "$NOWORKSHOP" == "true" ]; then
		start_command="$start_command -noworkshop"
	fi
	if [ "$NOADDONS" == "true" ]; then
		start_command="$start_command -noaddons"
	fi
	if [ "${DISABLELUAREFRESH:-true}" == "true" ]; then
		start_command="$start_command -disableluarefresh"
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
