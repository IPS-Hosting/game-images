#!/bin/bash

set -o errexit
set -o pipefail

# Prints an error to stderr, prefixed by [ips-error] and exits the programm with code 1.
function error() {
	echo >&2 "[ips-error] $1"
	exit 1
}

function apply_fixes() {
	# Fixes: [S_API FAIL] SteamAPI_Init() failed; unable to locate a running instance of Steam,or a local steamclient.so.
	ensure_steamcmd
	if [ ! -f "/home/ips-hosting/.steam/sdk32/steamclient.so" ]; then
		mkdir -vp /home/ips-hosting/.steam/sdk32
		cp -v /ips-hosting/steamcmd/linux32/steamclient.so /home/ips-hosting/.steam/sdk32/steamclient.so
	fi
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

# Downloads mods from steam and symlinks them to /home/ips-hosting/mods.
function install_mods() {
	if [ -n "$MANAGED_MODS" ]; then
		echo "Installing mods..."

		# Create mods array from space separated list of mods in env variable.
		readarray -d ' ' -t MANAGED_MODS_ARRAY < <(printf '%s' "$MANAGED_MODS")

		# Create array that contains the path of all installed mods.
		readarray -d '' INSTALLED_MODS_ARRAY < <(find /home/ips-hosting/Steam/steamapps/workshop/content/107410 -mindepth 1 -maxdepth 1 -type d -print0)

		echo "Removing old mods..."
		for INSTALLED_MOD_PATH in "${INSTALLED_MODS_ARRAY[@]}"; do
			INSTALLED_MOD=$(basename "$INSTALLED_MOD_PATH")

			if [[ ! "${MANAGED_MODS_ARRAY[*]}" == *"${INSTALLED_MOD}"* ]]; then
				rm -vrf "$INSTALLED_MOD_PATH"
			fi
		done
		echo "Finished removing old logs"

		echo "Downloading mods..."
		ensure_steamcmd
		cd /ips-hosting/steamcmd
		# shellcheck disable=SC2046
		./steamcmd.sh +login "$STEAM_USERNAME" "$STEAM_PASSWORD"$(printf " +workshop_download_item 107410 %s" "${MANAGED_MODS_ARRAY[@]}") validate +quit
		echo "Finished downloading mods"

		echo "Symlinking all mods..."
		mkdir -vp /home/ips-hosting/mods
		for modid in "${MANAGED_MODS_ARRAY[@]}"; do
			if [ ! -d "/home/ips-hosting/Steam/steamapps/workshop/content/107410/${modid}" ]; then
				echo "Warning! '/home/ips-hosting/Steam/steamapps/workshop/content/107410/${modid}' does not exist"
			elif [ ! -L "/home/ips-hosting/mods/${modid}" ]; then
				ln -sv "/home/ips-hosting/Steam/steamapps/workshop/content/107410/${modid}" "/home/ips-hosting/mods/${modid}"
			else
				echo "${modid} is already symlinked"
			fi
		done
		echo "Finished symlinking all mods"

		echo "Clearing broken symlinks..."
		find /home/ips-hosting/mods -xtype l -delete -print
		echo "Finished clearing broken symlinks"

		echo "Mod installation succeeded"
	fi
}

function update_validate() {
	ensure_steamcmd
	cd /ips-hosting/steamcmd

	if [ -n "${BETA_BRANCH}" ] && [ -n "${BETA_PASSWORD}" ]; then
		./steamcmd.sh +login "$STEAM_USERNAME" "$STEAM_PASSWORD" +force_install_dir /home/ips-hosting +app_update 233780 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" validate +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +login "$STEAM_USERNAME" "$STEAM_PASSWORD" +force_install_dir /home/ips-hosting +app_update 233780 -beta "${BETA_BRANCH}" validate +quit
	else
		./steamcmd.sh +login "$STEAM_USERNAME" "$STEAM_PASSWORD" +force_install_dir /home/ips-hosting +app_update 233780 validate +quit
	fi

	install_mods
	apply_fixes
}

function update() {
	ensure_steamcmd
	cd /ips-hosting/steamcmd

	if [ -n "${BETA_BRANCH}" ] && [ -n "${BETA_PASSWORD}" ]; then
		./steamcmd.sh +login "$STEAM_USERNAME" "$STEAM_PASSWORD" +force_install_dir /home/ips-hosting +app_update 233780 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +login "$STEAM_USERNAME" "$STEAM_PASSWORD" +force_install_dir /home/ips-hosting +app_update 233780 -beta "${BETA_BRANCH}" +quit
	else
		./steamcmd.sh +login "$STEAM_USERNAME" "$STEAM_PASSWORD" +force_install_dir /home/ips-hosting +app_update 233780 +quit
	fi

	install_mods
	apply_fixes
}

function extract_mod_keys() {
	echo "Extracting mod signature keys..."
	mkdir -vp /home/ips-hosting/mods
	mkdir -vp /home/ips-hosting/keys
	find -L /home/ips-hosting/mods -type f -name \*.bikey -exec sh -c 'modKey="$(readlink -f $1)"; cp -vf $modKey /home/ips-hosting/keys' _ {} \;
}

function patch_mods() {
	mkdir -vp /home/ips-hosting/mods

	# https://community.bistudio.com/wiki/Arma_3_Dedicated_Server#Case_sensitivity_.26_Mods
	echo "Converting all files in the mods directory to lowercase..."
	find -L /home/ips-hosting/mods -depth -exec rename -f -v 's/(.*)\/([^\/]*)/$1\/\L$2/' {} \;

	echo "Replacing spaces by underscores in mods folders..."
	find /home/ips-hosting/mods -maxdepth 1 -type d -exec rename -f -v 's/\s/_/g' {} \;
}

function start() {
	patch_mods

	local start_command=""
	if [ "${USE_X64:-true}" == "true" ]; then
		start_command="./arma3server_x64"
	else
		start_command="./arma3server"
	fi

	case "${MODE:-server}" in
	server)
		start_command="$start_command -ip=${HOST} -port=${GAME_PORT:-2302} -name='${PROFILE:-server}' -cfg='${BASIC_CFG:-basic.cfg}' -config='${SERVER_CFG:-server.cfg}' -bepath=/home/ips-hosting/battleye -mod='${MODS}' -serverMod='${SERVER_MODS}'"
		if [ "$AUTO_INIT" == "true" ]; then
			start_command="$start_command -autoInit"
		fi
		if [ "$LOAD_MISSION_TO_MEMORY" ]; then
			start_command="$start_command -loadMissionToMemory"
		fi

		if [ "$EXTRACT_MOD_KEYS" = true ]; then
			extract_mod_keys
		fi
		;;
	client)
		start_command="$start_command -client -connect=${GAME_SERVER_IP} -port=${GAME_SERVER_PORT:-2302} -password='${GAME_SERVER_PASSWORD}' -name='${PROFILE:-server}' -mod='${MODS}'"
		;;
	*)
		error "unknown mode: ${MODE}"
		;;
	esac
	
	
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
