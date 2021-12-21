function Ensure-Installation {
    if (!(Test-Path -Path "C:/arma3server/steamcmd/steamcmd.exe" -PathType leaf)) {
        New-Item -Path "C:/arma3server" -Name "steamcmd" -ItemType "directory"

        $(cd "steamcmd")

        $(curl.exe -L -o steamcmd.zip https://steamcdn-a.akamaihd.net/client/installer/steamcmd.zip)

        Expand-Archive ./steamcmd.zip .

        Start-Process "C:/arma3server/steamcmd/steamcmd.exe" -ArgumentList "+login anonymous +quit" -Wait -NoNewWindow

        Write-Output "SteamCMD Setup complete"
    }

    if (!(Test-Path -Path "C:/arma3server/X3DAudio1_7.dll" -PathType leaf)) {
        Copy-Item "C:/temp/X3DAudio1_7.dll" "C:/arma3server/X3DAudio1_7.dll"
        Write-Output "Copied 3DAudio"
    }
    if (!(Test-Path -Path "C:/arma3server/XAPOFX1_5.dll" -PathType leaf)) {
        Copy-Item "C:/temp/XAPOFX1_5.dll" "C:/arma3server/XAPOFX1_5.dll"
        Write-Output "Copied APOFX"
    }
}

function error($1) {
    Write-Error "[ips-error] $1"
}

function Get-LatestRpt()
{
    $latestRpt = $null

    try 
    {
        $latestRpt = Get-ChildItem "logfiles" -Filter "*.rpt" `
                    | Sort-Object LastWriteTime -Descending `
                    | Select-Object -First 1
    }
    catch { }

    return $latestRpt
}

function Check-SteamCredentials {
    if (($null -eq $env:STEAM_USERNAME) -or ($null -eq $env:STEAM_PASSWORD)) {
        error "Missing steam login information"

        exit 1
    }
}

function Install-Mods {
    if ($env:MANAGED_MODS -ne "") {
        Write-Output "Installing mods..."

        $MANAGED_MODS_ARRAY = $env:MANAGED_MODS.split(" ")

        if (Test-Path -Path "C:/arma3server/steamcmd/steamapps/workshop/content/107410" -PathType container) {

            $INSTALLED_MODS_ARRAY = Get-ChildItem "C:/arma3server/steamcmd/steamapps/workshop/content/107410" | Where-Object {$_.PSIsContainer} | Foreach-Object {$_.Name}

            Write-Output "Removing old mods..."
            foreach ($folderName in $INSTALLED_MODS_ARRAY) {
                if (!($folderName -in $MANAGED_MODS_ARRAY)) {
                    Remove-Item "C:/arma3server/steamcmd/steamapps/workshop/content/107410/$folderName" -Recurse
                }
            }
            Write-Output "Finished removing old mods"
        }

        if ($MANAGED_MODS_ARRAY.Length -gt 0) {
            if (!(Test-Path -Path "C:/arma3server/mods" -PathType container)) {
                New-Item -Path "C:/arma3server" -Name "mods" -ItemType "directory"
            }

            Ensure-Installation
    
            :modLoop foreach ($modid in $MANAGED_MODS_ARRAY) {
                $attempt = 1
                :downloadTrier while ($true) {
                    # SteamCMD will fail / timeout for big mods. The good thing is that it records what has been done,
					# so we can just try again and it will continue where it left off.
					# Downloading huge mods might take a few attempts.
                    Write-Output "Downloading mod $modid (attempt $attempt)..."
                    $steamcmdProcess = (Start-Process -FilePath "C:/arma3server/steamcmd/steamcmd.exe" -ArgumentList "+login $env:STEAM_USERNAME $env:STEAM_PASSWORD +workshop_download_item 107410 $modid +quit" -Wait -PassThru)
                    switch ($steamcmdProcess) {
                        0 {break downloadTrier}
                        default {
                            $attempt = $attempt + 1
                            if ($attempt -le 5) {
                                Write-Output "Download of $modid failed, retrying"
                            } else {
                                Write-Output "Download failed for the 5th time, skipping mod"
                                continue modLoop
                            }
                        }
                    }
                }

                if (!(Test-Path -Path "C:/arma3server/steamcmd/steamapps/workshop/content/107410/$modid" -PathType container)) {
                    Write-Output 'WARNING!'
                    Write-Output "C:/arma3server/steamcmd/steamapps/workshop/content/107410/$modid does not exist"
                } elseif (!(Test-Path -Path "C:/arma3server/mods/$modid" -PathType any)) {
                    New-Item -ItemType "junction" -Path "C:/arma3server/mods/$modid" -Target "C:/arma3server/steamcmd/steamapps/workshop/content/107410/$modid"
                } else {
                    Write-Output "$modid is already symlinked"
                }
            }

            # Clear broken symlinks (e.g. symlinks to old mods).
            if (Test-Path -Path "C:/arma3server/mods" -PathType container) {
                Write-Output "Clearing broken symlinks..."
                    $links = Get-ChildItem -Path "C:/arma3server/mods" -Force | Where-Object { $_.LinkType -ne $null -or $_.Attributes -match "ReparsePoint" }
                    foreach ($link in $links) {
                        if (!(Test-Path -Path $link.Target -PathType container)) {
                            Remove-Item $link.FullName
                        }
                    }
                Write-Output "Finished clearing broken symlinks"
            }

            Write-Output "Mod installation succeeded"
        }
    }
}

function Invoke-Update {
    Check-SteamCredentials
    Ensure-Installation
    if (($null -ne $env:BETA_BRANCH) -and ($null -ne $env:BETA_PASSWORD)) {
        Start-Process -FilePath "C:/arma3server/steamcmd/steamcmd.exe" -ArgumentList "+force_install_dir C:/arma3server +login $env:STEAM_USERNAME $env:STEAM_PASSWORD +app_update 233780 -beta $env:BETA_BRANCH -betapassword $env:BETA_PASSWORD +quit" -Wait -NoNewWindow
    } elseif (($null -ne $env:BETA_BRANCH)) {
        Start-Process -FilePath "C:/arma3server/steamcmd/steamcmd.exe" -ArgumentList "+force_install_dir C:/arma3server +login $env:STEAM_USERNAME $env:STEAM_PASSWORD +app_update 233780 -beta $env:BETA_BRANCH +quit" -Wait -NoNewWindow
    } else {
        Start-Process -FilePath "C:/arma3server/steamcmd/steamcmd.exe" -ArgumentList "+force_install_dir C:/arma3server +login $env:STEAM_USERNAME $env:STEAM_PASSWORD +app_update 233780 +quit" -Wait -NoNewWindow
    }
    Install-Mods
}

function Invoke-UpdateValidate {
    Check-SteamCredentials
    Ensure-Installation
    if (($null -ne $env:BETA_BRANCH) -and ($null -ne $env:BETA_PASSWORD)) {
        Start-Process -FilePath "C:/arma3server/steamcmd/steamcmd.exe" -ArgumentList "+force_install_dir C:/arma3server +login $env:STEAM_USERNAME $env:STEAM_PASSWORD +app_update 233780 -beta $env:BETA_BRANCH -betapassword $env:BETA_PASSWORD validate +quit" -Wait -NoNewWindow
    } elseif (($null -ne $env:BETA_BRANCH)) {
        Start-Process -FilePath "C:/arma3server/steamcmd/steamcmd.exe" -ArgumentList "+force_install_dir C:/arma3server +login $env:STEAM_USERNAME $env:STEAM_PASSWORD +app_update 233780 -beta $env:BETA_BRANCH validate +quit" -Wait -NoNewWindow
    } else {
        Start-Process -FilePath "C:/arma3server/steamcmd/steamcmd.exe" -ArgumentList "+force_install_dir C:/arma3server +login $env:STEAM_USERNAME $env:STEAM_PASSWORD +app_update 233780 validate +quit" -Wait -NoNewWindow
    }
    Install-Mods
}


function Extract-ModKeys {
    Write-Output "Extracting mod signature keys..."
    if (!(Test-Path -Path "C:/arma3server/mods" -PathType container)) {
        New-Item -Path "C:/arma3server" -Name "mods" -ItemType "directory"
    }

    if (!(Test-Path -Path "C:/arma3server/keys" -PathType container)) {
        New-Item -Path "C:/arma3server" -Name "keys" -ItemType "directory"
    }

    Get-ChildItem "C:/arma3server/mods" -Depth 4 -Filter *.bikey | Copy-Item -Destination "C:/arma3server/keys" -Force
}

function Invoke-Start {
    $Start_Options = ""

    switch ($env:MODE) {
        client {
            $Start_Options = "-client -connect=$env:GAME_SERVER_IP -port=$env:GAME_SERVER_PORT -password=$env:GAME_SERVER_PASSWORD -name=$env:PROFILE -mod=$env:MODS -limitFPS=$env:LIMIT_FPS -profiles=logfiles"
        }

        default {
            $Start_Options = "-ip=$env:HOST -port=$env:GAME_PORT -name=$env:PROFILE -cfg=$env:BASIC_CFG -config=$env:SERVER_CFG -mod=$env:MODS -serverMod=$env:SERVER_MODS -limitFPS=$env:LIMIT_FPS -profiles=logfiles"

            if ($env:AUTO_INIT -eq $true) {
                $Start_Options = "$Start_Options -autoInit"
            }

            if ($env:LOAD_MISSION_TO_MEMORY -eq $true) {
                $Start_Options = "$Start_Options -loadMissionToMemory"
            }

            if ($env:EXTRACT_MOD_KEYS -eq $true) {
                Extract-ModKeys
            }
        }
    }

    if ($false -eq $env:USE_X64) {
        Write-Output "C:/arma3server/arma3server_x64.exe $Start_Options"
        Start-Process -FilePath "C:/arma3server/arma3server_x64.exe" -ArgumentList $Start_Options
    } else {
        Write-Output "C:/arma3server/arma3server.exe $Start_Options"
        Start-Process -FilePath "C:/arma3server/arma3server.exe" -ArgumentList $Start_Options
    }
    
    $rpt = Get-LatestRpt
    Get-Content $rpt.FullName -Wait
}

switch ($args[0]) {
    update {Invoke-Update}
    update_validate { Invoke-UpdateValidate}
    start { Invoke-Start }
    default {
        Invoke-UpdateValidate
        Invoke-Start
    }
}