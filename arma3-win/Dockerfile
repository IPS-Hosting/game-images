FROM mcr.microsoft.com/windows/server:ltsc2022

ENV HOST=0.0.0.0
ENV GAME_PORT=2302
ENV PROFILE=server
ENV BASIC_CFG=basic.cfg
ENV SERVER_CFG=server.cfg
ENV LIMIT_FPS=50

COPY "./entrypoint.ps1" "C:/arma3/entrypoint.ps1"
 
COPY "./X3DAudio1_7.dll" "C:/arma3/temp/X3DAudio1_7.dll"
COPY "./XAPOFX1_5.dll" "C:/arma3/temp/XAPOFX1_5.dll"

COPY "./vcredist_x64.exe" "C:/arma3/temp/vcredist_x64.exe"

RUN powershell.exe -Command Start-Process "C:/arma3/temp/vcredist_x64.exe" -ArgumentList '/quiet' -Wait

WORKDIR "C:/arma3/arma3server"
VOLUME "C:/arma3/arma3server"

EXPOSE 2302/udp
EXPOSE 2303/udp
EXPOSE 2304/udp
EXPOSE 2305/udp
EXPOSE 2306/udp

ENTRYPOINT ["powershell", "-Command", "C:/arma3/entrypoint.ps1"]