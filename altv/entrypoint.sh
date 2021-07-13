#!/bin/bash

set -o errexit
set -o pipefail

# Downloads the alt:V updater, overwriting any previous or potentially modified version.
function ensure_altv_updater() {
  mkdir -vp /home/ips-hosting
	cd /home/ips-hosting
	wget -vO 'update.sh' 'https://raw.githubusercontent.com/Lhoerion/altv-serverupdater/master/update.sh'
	chmod +x ./update.sh
}

function install_update() {
  echo "Ensuring the alt:V updater is installed and up to date"
  ensure_altv_updater
  cd /home/ips-hosting

  echo "Ensuring server files are installed and up to date"
  ./update.sh

  if [ -f "package-lock.json" ]; then
		echo "Installing dependencies using npm ci"
  	npm ci
	elif [ -f "yarn.lock" ]; then
		if [ -d ".yarn" ]; then
			echo "Installing dependencies using yarn install --immutable"
			yarn install --immutable
		else
			echo "Installing dependencies using yarn install --frozen-lockfile"
			yarn install --frozen-lockfile
		fi
	fi

	if [ -f "package.json" ]; then
		echo "Running npm build script if it exists"
		npm run build --if-present
	fi
}

function start() {
  cd /home/ips-hosting
  ./altv-server --config ./server.cfg --host "${HOST:-0.0.0.0}" --port "${PORT:-7788}"
}

case "$1" in
install_update)
  install_update
  ;;
start)
  start
  ;;
*)
  install_update
  start
  ;;
esac
