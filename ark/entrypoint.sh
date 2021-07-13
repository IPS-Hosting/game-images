#!/bin/bash

set -o errexit
set -o pipefail

# Requires steamcmd to be mounted.
function apply_fixes() {
	# Fixes: [S_API FAIL] SteamAPI_Init() failed; unable to locate a running instance of Steam,or a local steamclient.so.
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

function ensure_steamcmd_ark() {
	mkdir -vp /home/ips-hosting/Engine/Binaries/ThirdParty/SteamCMD/Linux
	cd /home/ips-hosting/Engine/Binaries/ThirdParty/SteamCMD/Linux

	if [ ! -f "./steamcmd.sh" ]; then
		wget https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz
		tar -xvzf steamcmd_linux.tar.gz
		rm steamcmd_linux.tar.gz
	fi
}

# Extracts a mod from steamcmd to ShooterGame/Content/Mods.
# Credits: https://github.com/FezVrasta/ark-server-tools
function extract_mod() {
	local modid=$1
	local steamworkshopdir="/home/ips-hosting/Steam/steamapps/workshop"
	local modsrcdir="${steamworkshopdir}/content/346110/$modid"
	local modextractdir="/home/ips-hosting/ShooterGame/Content/Mods/$modid"
	# The linux branch for mods is often broken and the windows branch is also usable on linux servers.
	local modbranch="Windows"

	# Bypass the 111111111 modid used by Primitive+
	if [ "$modid" = "111111111" ]; then
		return 0
	fi

	if [ -f "$modsrcdir/mod.info" ]; then
		echo "Extracting files to $modextractdir"
		if [ -f "$modsrcdir/${modbranch}NoEditor/mod.info" ]; then
			modsrcdir="$modsrcdir/${modbranch}NoEditor"
		fi
		find "$modsrcdir" -type d -printf "$modextractdir/%P\0" | xargs -0 -r mkdir -p
		find "$modextractdir" -type f ! -name '.*' -printf "%P\n" | while read -r f; do
			if [ ! -f "$modsrcdir/$f" ] && [ ! -f "$modsrcdir/${f}.z" ]; then
				rm "$modextractdir/$f"
			fi
		done
		find "$modextractdir" -depth -type d -printf "%P\n" | while read -r d; do
			if [ ! -d "$modsrcdir/$d" ]; then
				rmdir "$modextractdir/$d"
			fi
		done
		find "$modsrcdir" -type f ! \( -name '*.z' -or -name '*.z.uncompressed_size' \) -printf "%P\n" | while read -r f; do
			if [ ! -f "$modextractdir/$f" ] || [ "$modsrcdir/$f" -nt "$modextractdir/$f" ]; then
				cp "$modsrcdir/$f" "$modextractdir/$f"
			fi
		done
		find "$modsrcdir" -type f -name '*.z' -printf "%P\n" | while read -r f; do
			if [ ! -f "$modextractdir/${f%.z}" ] || [ "$modsrcdir/$f" -nt "$modextractdir/${f%.z}" ]; then
				perl -M'Compress::Raw::Zlib' -e '
					my $sig;
					read(STDIN, $sig, 8) or die "Unable to read compressed file: $!";
					if ($sig != "\xC1\x83\x2A\x9E\x00\x00\x00\x00"){
					die "Bad file magic";
					}
					my $data;
					read(STDIN, $data, 24) or die "Unable to read compressed file: $!";
					my ($chunksizelo, $chunksizehi,
						$comprtotlo,  $comprtothi,
						$uncomtotlo,  $uncomtothi)  = unpack("(LLLLLL)<", $data);
					my @chunks = ();
					my $comprused = 0;
					while ($comprused < $comprtotlo) {
					read(STDIN, $data, 16) or die "Unable to read compressed file: $!";
					my ($comprsizelo, $comprsizehi,
						$uncomsizelo, $uncomsizehi) = unpack("(LLLL)<", $data);
					push @chunks, $comprsizelo;
					$comprused += $comprsizelo;
					}
					foreach my $comprsize (@chunks) {
					read(STDIN, $data, $comprsize) or die "File read failed: $!";
					my ($inflate, $status) = new Compress::Raw::Zlib::Inflate();
					my $output;
					$status = $inflate->inflate($data, $output, 1);
					if ($status != Z_STREAM_END) {
						die "Bad compressed stream; status: " . ($status);
					}
					if (length($data) != 0) {
						die "Unconsumed data in input"
					}
					print $output;
					}
				' <"$modsrcdir/$f" >"$modextractdir/${f%.z}"
				touch -c -r "$modsrcdir/$f" "$modextractdir/${f%.z}"
			fi
		done
		modname="$(curl -s "http://steamcommunity.com/sharedfiles/filedetails/?id=${modid}" | sed -n 's|^.*<div class="workshopItemTitle">\([^<]*\)</div>.*|\1|p')"
		if [ -f "${modextractdir}/.mod" ]; then
			rm "${modextractdir}/.mod"
		fi
		perl -e '
			my $data;
			{ local $/; $data = <STDIN>; }
			my $mapnamelen = unpack("@0 L<", $data);
			my $mapname = substr($data, 4, $mapnamelen - 1);
			my $nummaps = unpack("@" . ($mapnamelen + 4) . " L<", $data);
			my $pos = $mapnamelen + 8;
			my $modname = ($ARGV[2] || $mapname) . "\x00";
			my $modnamelen = length($modname);
			my $modpath = "../../../" . $ARGV[0] . "/Content/Mods/" . $ARGV[1] . "\x00";
			my $modpathlen = length($modpath);
			print pack("L< L< L< Z$modnamelen L< Z$modpathlen L<",
			$ARGV[1], 0, $modnamelen, $modname, $modpathlen, $modpath,
			$nummaps);
			for (my $mapnum = 0; $mapnum < $nummaps; $mapnum++){
			my $mapfilelen = unpack("@" . ($pos) . " L<", $data);
			my $mapfile = substr($data, $mapnamelen + 12, $mapfilelen);
			print pack("L< Z$mapfilelen", $mapfilelen, $mapfile);
			$pos = $pos + 4 + $mapfilelen;
			}
			print "\x33\xFF\x22\xFF\x02\x00\x00\x00\x01";
		' "ShooterGame" "$modid" "$modname" <"$modextractdir/mod.info" >"${modextractdir}.mod"
		if [ -f "$modextractdir/modmeta.info" ]; then
			cat "$modextractdir/modmeta.info" >>"${modextractdir}.mod"
		else
			echo -ne '\x01\x00\x00\x00\x08\x00\x00\x00ModType\x00\x02\x00\x00\x001\x00' >>"${modextractdir}.mod"
		fi
	fi
}

# Ensures all mods are installed and up to date.
function download_mods() {
	if [ -n "$MANAGED_MODS" ]; then
		echo "Downloading mods"

		# Create mods array from space separated list of mods in env variable.
		readarray -d ' ' -t MANAGED_MODS_ARRAY < <(printf '%s' "$MANAGED_MODS")

		# Download all mods.
		ensure_steamcmd_ark
		cd /home/ips-hosting/Engine/Binaries/ThirdParty/SteamCMD/Linux
		# shellcheck disable=SC2046
		./steamcmd.sh +login anonymous$(printf " +workshop_download_item 346110 %s" "${MANAGED_MODS_ARRAY[@]}") validate +quit

		# Extract all mods.
		for modid in "${MANAGED_MODS_ARRAY[@]}"; do
			extract_mod "$modid"
		done

		echo "Finished downloading mods"
	fi
}

function update_validate() {
	ensure_steamcmd
	cd /ips-hosting/steamcmd

	if [ -n "${BETA_BRANCH}" ] && [ -n "${BETA_PASSWORD}" ]; then
		./steamcmd.sh +login anonymous +force_install_dir /home/ips-hosting +app_update 376030 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" validate +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +login anonymous +force_install_dir /home/ips-hosting +app_update 376030 -beta "${BETA_BRANCH}" validate +quit
	else
		./steamcmd.sh +login anonymous +force_install_dir /home/ips-hosting +app_update 376030 validate +quit
	fi

	apply_fixes
	download_mods
}

function update() {
	ensure_steamcmd
	cd /ips-hosting/steamcmd

	if [ -n "${BETA_BRANCH}" ] && [ -n "${BETA_PASSWORD}" ]; then
		./steamcmd.sh +login anonymous +force_install_dir /home/ips-hosting +app_update 376030 -beta "${BETA_BRANCH}" -betapassword "${BETA_PASSWORD}" +quit
	elif [ -n "$BETA_BRANCH" ]; then
		./steamcmd.sh +login anonymous +force_install_dir /home/ips-hosting +app_update 376030 -beta "${BETA_BRANCH}" +quit
	else
		./steamcmd.sh +login anonymous +force_install_dir /home/ips-hosting +app_update 376030 +quit
	fi

	apply_fixes
	download_mods
}

# Starts the game server.
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
	start_command="$start_command -server -crossplay -PublicIPForEpic=$HOST"
	if [ "$SERVER_GAME_LOG" == "true" ]; then
		start_command="$start_command -servergamelog"
	fi
	if [ "$SERVER_GAME_LOG_INCLUDE_TRIBE_LOGS" == "true" ]; then
		start_command="$start_command -servergamelogincludetribelogs"
	fi
	if [ "$FORCE_ALLOW_CAVE_FYLERS" == "true" ]; then
		start_command="$start_command -ForceAllowCaveFlyers"
	fi

	cd /home/ips-hosting/ShooterGame/Binaries/Linux
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
