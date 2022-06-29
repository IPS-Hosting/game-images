FROM mcr.microsoft.com/windows/servercore:ltsc2022

ENV HOST=0.0.0.0
ENV GAME_PORT=2302
ENV PROFILE=server
ENV BASIC_CFG=basic.cfg
ENV SERVER_CFG=server.cfg
ENV LIMIT_FPS=50

COPY "./entrypoint.ps1" "C:/arma3/entrypoint.ps1"
 
#Install Chocolatey
RUN powershell.exe -Command "Set-ExecutionPolicy Bypass -Scope Process -Force; [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072; iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))"

RUN choco install dotnet4.5.2 -y

RUN choco install vcredist2010 -y

RUN choco install vcredist2013 -y

WORKDIR "C:/arma3/arma3server"
VOLUME "C:/arma3/arma3server"

EXPOSE 2302/udp
EXPOSE 2303/udp
EXPOSE 2304/udp
EXPOSE 2305/udp
EXPOSE 2306/udp

ENTRYPOINT ["powershell", "-Command", "C:/arma3/entrypoint.ps1"]