#!/bin/bash

set -o errexit
set -o pipefail

CREDENTIALS_PATH="${CREDENTIALS_PATH:-/home/ips-hosting/.hytale-downloader-credentials.json}"
PATCHLINE="${PATCHLINE:-release}"
SKIP_UPDATE_CHECK="${SKIP_UPDATE_CHECK:-false}"
INSTALL_DEFAULT_PLUGINS="${INSTALL_DEFAULT_PLUGINS:-true}"
DEFAULT_PLUGINS="${DEFAULT_PLUGINS:-webserver,query,performance-saver,prometheus}"

function install_update_launcher() {
	mkdir -vp /home/ips-hosting/hytale-launcher
	cd /home/ips-hosting/hytale-launcher

	local launcher_was_just_installed=false

	# Install launcher if it doesn't exist
	if [ ! -f "./hytale-downloader-linux-amd64" ]; then
		echo "Downloading hytale-downloader..."
		wget -q https://downloader.hytale.com/hytale-downloader.zip
		unzip ./hytale-downloader.zip hytale-downloader-linux-amd64
		rm hytale-downloader.zip
		launcher_was_just_installed=true
	fi

	# Check for launcher updates unless explicitly disabled or just installed
	if [ "$SKIP_UPDATE_CHECK" != "true" ] && [ "$launcher_was_just_installed" = "false" ]; then
		local update_check_output
		update_check_output=$(./hytale-downloader-linux-amd64 -check-update 2>&1)
		
		# Check if update is available by looking for the version message
		if echo "$update_check_output" | grep -q "new version.*is available"; then
			echo "$update_check_output"
			echo "Updating hytale-downloader..."
			
			# Download and install the new version
			wget -q https://downloader.hytale.com/hytale-downloader.zip
			unzip -o ./hytale-downloader.zip hytale-downloader-linux-amd64
			rm hytale-downloader.zip
			
			echo "hytale-downloader updated successfully"
		else
			# Still show the output to indicate launcher is up to date
			echo "$update_check_output"
		fi
	fi
}

function get_current_game_version() {
	cd /home/ips-hosting
	
	if [ ! -f "./Server/HytaleServer.jar" ]; then
		echo ""
		return
	fi
	
	# Try to get version from server jar
	local version_output
	if version_output=$(java -XX:AOTCache=./Server/HytaleServer.aot -jar ./Server/HytaleServer.jar --version 2>&1); then
		# Sample output: HytaleServer v2026.01.13-50e69c385 (release)
		# Extract version
		echo "$version_output" | grep -oP '[0-9]+\.[0-9]+\.[0-9]+(?:-[a-zA-Z0-9]+)?' || echo ""
	else
		echo ""	
	fi
}

function get_latest_game_version() {
	cd /home/ips-hosting
	
	# Get latest version from launcher
	local version
	version=$(./hytale-launcher/hytale-downloader-linux-amd64 -print-version -patchline "${PATCHLINE}" 2>/dev/null || echo "")
	if [ -n "$version" ]; then
		echo "$version"
	else
		echo ""
	fi
}

function needs_update() {
	local current_version
	local latest_version
	
	current_version=$(get_current_game_version)
	latest_version=$(get_latest_game_version)
	
	# If we can't get current version (doesn't exist or error), we need to update
	if [ -z "$current_version" ]; then
		echo "Warning: Could not determine current version"
		return 0
	fi
	
	# If we can't get latest version, skip update
	if [ -z "$latest_version" ]; then
		echo "Warning: Could not determine latest version"
		return 1
	fi
	
	# Compare versions
	if [ "$current_version" != "$latest_version" ]; then
		echo "Update available: $current_version -> $latest_version"
		return 0
	fi
	
	echo "Server is up to date: $current_version"
	return 1
}

function install_update_game() {
	if [ "$FORCE_UPDATE" != "true" ] && ! needs_update; then
		return
	fi

	cd /home/ips-hosting

	./hytale-launcher/hytale-downloader-linux-amd64 \
		-credentials-path "${CREDENTIALS_PATH:-/home/ips-hosting/.hytale-downloader-credentials.json}" \
		-download-path "/home/ips-hosting/${PATCHLINE}.zip" \
		-patchline "${PATCHLINE}"
	
	unzip -o "/home/ips-hosting/${PATCHLINE}.zip"
	rm "/home/ips-hosting/${PATCHLINE}.zip"
}

function install_default_plugins() {
	# Optionally install default plugins into mods/
	if [ "$INSTALL_DEFAULT_PLUGINS" != "true" ]; then
		return
	fi

	local mods_dir="/home/ips-hosting/mods"
	mkdir -p "$mods_dir"

	# Helper: get latest release JAR URL for a repo
	function _latest_jar_url() {
		local repo="$1"
		# Fetch latest release and pick first .jar asset URL
		curl -s "https://api.github.com/repos/${repo}/releases/latest" \
			| grep -Eo '"browser_download_url": "[^"]+\.jar"' \
			| head -n 1 \
			| cut -d '"' -f4
	}

	# Install a plugin by repo
	function _install_plugin_repo() {
		local repo="$1"
		local plugin_name
		plugin_name=$(basename "$repo")
		
		local url
		url=$(_latest_jar_url "$repo")
		if [ -z "$url" ]; then
			echo "Warning: Could not find release JAR for $repo"
			return
		fi
		
		local filename
		filename=$(basename "$url")
		
		# Check if this exact version is already installed
		if [ -f "$mods_dir/$filename" ]; then
			echo "Plugin $filename already installed, skipping download"
			return
		fi
		
		# Remove old versions of the plugin
		rm -fv "$mods_dir"/*"$plugin_name"*.jar
		
		echo "Downloading $repo -> $filename"
		wget -q -O "$mods_dir/$filename" "$url"
	}

	# Parse DEFAULT_PLUGINS comma-separated list
	IFS=',' read -ra _plugins <<< "$DEFAULT_PLUGINS"
	for p in "${_plugins[@]}"; do
		case "$(echo "$p" | tr '[:upper:]' '[:lower:]' | xargs)" in
			webserver)
				_install_plugin_repo "nitrado/hytale-plugin-webserver"
				;;
			query)
				_install_plugin_repo "nitrado/hytale-plugin-query"
				;;
			performance-saver)
				_install_plugin_repo "nitrado/hytale-plugin-performance-saver"
				;;
			prometheus)
				_install_plugin_repo "apexhosting/hytale-plugin-prometheus"
				;;
			"")
				;;
			*)
				echo "Warning: Unknown plugin '$p'"
				;;
		esac
	done
}

function ensure_machine_id() {
	# The /etc/machine-id file is a unique identifier for the system.
	# It's required by Hytale for hardware UUID detection.
	# Docker containers typically don't have this file by default, so we need to create it.
	# We generate it once and persist it in /home/ips-hosting to ensure the same ID
	# is used across container restarts, allowing Hytale to recognize the container
	# as the same machine even after restart.
	
	local persisted_machine_id_file="/home/ips-hosting/.machine-id"

	if [ ! -s "/etc/machine-id" ]; then
		if [ ! -s "$persisted_machine_id_file" ]; then
			# Generate a 32-character hex string in the correct machine-id format
			openssl rand -hex 16 > "$persisted_machine_id_file"
		fi
		cat "$persisted_machine_id_file" > /etc/machine-id
	fi
}


function start() {
	ensure_machine_id

	cd /home/ips-hosting

	# Java command
	local start_command="java"
	start_command="$start_command -XX:AOTCache=./Server/HytaleServer.aot"
	if [ -n "$JVM_ARGS" ]; then
		start_command="$start_command $JVM_ARGS"
	fi
	start_command="$start_command -jar ./Server/HytaleServer.jar"

	# Network binding
	start_command="$start_command --bind ${BIND:-0.0.0.0:${PORT:-5520}}"
	
	# Assets
	start_command="$start_command --assets ${ASSETS:-./Assets.zip}"
	
	# Authentication mode
	if [ -n "$AUTH_MODE" ]; then
		start_command="$start_command --auth-mode $AUTH_MODE"
	fi
	
	# Transport type
	if [ -n "$TRANSPORT" ]; then
		start_command="$start_command --transport $TRANSPORT"
	fi
	
	# Backup settings
	if [ "$BACKUP" == "true" ]; then
		start_command="$start_command --backup"
	fi
	if [ -n "$BACKUP_DIR" ]; then
		start_command="$start_command --backup-dir $BACKUP_DIR"
	fi
	if [ -n "$BACKUP_FREQUENCY" ]; then
		start_command="$start_command --backup-frequency $BACKUP_FREQUENCY"
	fi
	if [ -n "$BACKUP_MAX_COUNT" ]; then
		start_command="$start_command --backup-max-count $BACKUP_MAX_COUNT"
	fi
	
	# Owner settings
	if [ -n "$OWNER_NAME" ]; then
		start_command="$start_command --owner-name $OWNER_NAME"
	fi
	if [ -n "$OWNER_UUID" ]; then
		start_command="$start_command --owner-uuid $OWNER_UUID"
	fi
	
	# Session and identity
	if [ -n "$SESSION_TOKEN" ]; then
		start_command="$start_command --session-token $SESSION_TOKEN"
	fi
	if [ -n "$IDENTITY_TOKEN" ]; then
		start_command="$start_command --identity-token $IDENTITY_TOKEN"
	fi
	
	# Paths
	if [ -n "$UNIVERSE" ]; then
		start_command="$start_command --universe $UNIVERSE"
	fi
	if [ -n "$WORLD_GEN" ]; then
		start_command="$start_command --world-gen $WORLD_GEN"
	fi
	if [ -n "$PREFAB_CACHE" ]; then
		start_command="$start_command --prefab-cache $PREFAB_CACHE"
	fi
	if [ -n "$MODS" ]; then
		start_command="$start_command --mods $MODS"
	fi
	if [ -n "$EARLY_PLUGINS" ]; then
		start_command="$start_command --early-plugins $EARLY_PLUGINS"
	fi
	
	# Boot command
	if [ -n "$BOOT_COMMAND" ]; then
		start_command="$start_command --boot-command \"$BOOT_COMMAND\""
	fi
	
	# Migration settings
	if [ -n "$MIGRATE_WORLDS" ]; then
		start_command="$start_command --migrate-worlds $MIGRATE_WORLDS"
	fi
	if [ -n "$MIGRATIONS" ]; then
		start_command="$start_command --migrations $MIGRATIONS"
	fi
	
	# Logging
	if [ -n "$LOG" ]; then
		start_command="$start_command --log $LOG"
	fi
	
	# Client PID
	if [ -n "$CLIENT_PID" ]; then
		start_command="$start_command --client-pid $CLIENT_PID"
	fi
	
	# Network settings
	if [ -n "$FORCE_NETWORK_FLUSH" ]; then
		start_command="$start_command --force-network-flush $FORCE_NETWORK_FLUSH"
	fi
	
	# Validation options
	if [ "$VALIDATE_ASSETS" == "true" ]; then
		start_command="$start_command --validate-assets"
	fi
	if [ -n "$VALIDATE_PREFABS" ]; then
		start_command="$start_command --validate-prefabs $VALIDATE_PREFABS"
	fi
	if [ "$VALIDATE_WORLD_GEN" == "true" ]; then
		start_command="$start_command --validate-world-gen"
	fi
	
	# Boolean flags
	if [ "$ACCEPT_EARLY_PLUGINS" == "true" ]; then
		start_command="$start_command --accept-early-plugins"
	fi
	if [ "$ALLOW_OP" == "true" ]; then
		start_command="$start_command --allow-op"
	fi
	if [ "$BARE" == "true" ]; then
		start_command="$start_command --bare"
	fi
	if [ "$DISABLE_ASSET_COMPARE" == "true" ]; then
		start_command="$start_command --disable-asset-compare"
	fi
	if [ "$DISABLE_CPB_BUILD" == "true" ]; then
		start_command="$start_command --disable-cpb-build"
	fi
	if [ "$DISABLE_FILE_WATCHER" == "true" ]; then
		start_command="$start_command --disable-file-watcher"
	fi
	if [ "$DISABLE_SENTRY" == "true" ]; then
		start_command="$start_command --disable-sentry"
	fi
	if [ "$EVENT_DEBUG" == "true" ]; then
		start_command="$start_command --event-debug"
	fi
	if [ "$GENERATE_SCHEMA" == "true" ]; then
		start_command="$start_command --generate-schema"
	fi
	if [ "$SHUTDOWN_AFTER_VALIDATE" == "true" ]; then
		start_command="$start_command --shutdown-after-validate"
	fi
	if [ "$SINGLEPLAYER" == "true" ]; then
		start_command="$start_command --singleplayer"
	fi

	echo "$start_command"
	eval "$start_command"
}

case "$1" in
install_update_launcher)
	install_update_launcher
	;;
install_update_game)
	install_update_launcher
	install_update_game
	;;
start)
	start
	;;
*)
	# Default: install/update launcher and game, then start
	install_update_launcher
	if [ "$SKIP_UPDATE_CHECK" != "true" ]; then
		install_update_game
	fi
	install_default_plugins
	start
	;;
esac
