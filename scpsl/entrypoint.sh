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

CONFIG_FILE="/home/ips-hosting/.config/SCP Secret Laboratory/config/${GAME_PORT:-7777}/config_gameplay.txt"

# Function to update config
function update_config() {
	local key=$1
	local value=$2

	# Convert key to lowercase
	key=$(echo "$key" | tr '[:upper:]' '[:lower:]')

	# Check if the key exists in the file
	if grep -q "^$key:" "$CONFIG_FILE"; then
		# Key exists, update it
		sed -i "s/^$key:.*/$key: $value/" "$CONFIG_FILE"
	else
		# Key doesn't exist, add it
		echo "$key: $value" >> "$CONFIG_FILE"
	fi
}

function start() {
	cd /home/ips-hosting

	# Setup useful symlinks
	ln -svf ".config/SCP Secret Laboratory/ServerLogs/${GAME_PORT:-7777}" /home/ips-hosting/ServerLogs
	ln -svf ".config/SCP Secret Laboratory/LocalAdminLogs/${GAME_PORT:-7777}" /home/ips-hosting/LocalAdminLogs

	local start_command="./LocalAdmin ${GAME_PORT:-7777} --printStd --noSetCursor --config /home/ips-hosting/localadmin_config.txt --useDefault --logLengthLimit ${LOG_LENGTH_LIMIT:-1G} --gameLogs '/home/ips-hosting/.config/SCP Secret Laboratory/ServerLogs/${GAME_PORT:-7777}' --logs '/home/ips-hosting/.config/SCP Secret Laboratory/LocalAdminLogs/${GAME_PORT:-7777}'"

 	if [ ! -f "$CONFIG_FILE" ]; then
		echo "Error: config_gameplay.txt does not exist at $CONFIG_FILE. Running server for 30s to generate config..."

		# Ensure .config folder exists: https://github.com/northwood-studios/LocalAdmin-V2/issues/52
		mkdir -vp .config

		# Run server long enough to generate config
		echo "yes" | eval timeout 30s "$start_command" || true
	fi

	# Loop through all config environment variables and add to config_gameplay.txt
	for var in $(compgen -e); do
		# Convert the variable name to lowercase for comparison
		var_lower=$(echo "$var" | tr '[:upper:]' '[:lower:]')

		# Check for variables starting with CONFIG_ or config_
		if [[ $var_lower == config_* ]]; then
			# Extract the key name (remove CONFIG_ prefix)
			key=${var_lower#config_}
			# Get the value of the variable
			value=${!var}

			# Update the config file
			update_config "$key" "$value"
		fi
	done

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
