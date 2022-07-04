# We need to use the bigger Windows Server Image and can't use Windows Server Core or Nano Server,
# because of the ARMA 3 server requirements.
# https://docs.microsoft.com/en-us/virtualization/windowscontainers/manage-containers/container-base-images#choosing-a-base-image
FROM mcr.microsoft.com/windows/server:ltsc2022

ENV HOST=0.0.0.0
ENV GAME_PORT=2302
ENV PROFILE=server
ENV BASIC_CFG=basic.cfg
ENV SERVER_CFG=server.cfg
ENV LIMIT_FPS=50

COPY "./entrypoint.ps1" "C:/Users/ContainerUser/entrypoint.ps1"
 
# Install Chocolatey
RUN powershell.exe -Command "Set-ExecutionPolicy Bypass -Scope Process -Force; [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072; iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))"

# Install Arma 3 requirements
# https://arma3.com/requirements
RUN choco install dotnet4.5.2 -y
RUN choco install vcredist2010 -y
RUN choco install vcredist2013 -y
RUN choco install directx -y

# Run as unprivileged container user
# https://docs.microsoft.com/en-us/virtualization/windowscontainers/manage-containers/container-security#when-to-use-containeradmin-and-containeruser-user-accounts
USER ContainerUser

WORKDIR "C:/Users/ContainerUser/arma3server"
VOLUME "C:/Users/ContainerUser/arma3server"
VOLUME "C:/Users/ContainerUser/steamcmd"

EXPOSE 2302/udp
EXPOSE 2303/udp
EXPOSE 2304/udp
EXPOSE 2305/udp
EXPOSE 2306/udp

ENTRYPOINT ["powershell", "-Command", "C:/Users/ContainerUser/entrypoint.ps1"]
