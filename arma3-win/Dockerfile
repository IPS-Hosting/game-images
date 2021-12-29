FROM mcr.microsoft.com/windows/servercore:ltsc2022

RUN NET USER /add arma3

ENV HOST=0.0.0.0
ENV GAME_PORT=2302
ENV PROFILE=server
ENV BASIC_CFG=basic.cfg
ENV SERVER_CFG=server.cfg
ENV LIMIT_FPS=50

COPY "./entrypoint.ps1" "C:/Users/arma3/Desktop/entrypoint.ps1"
 
COPY "./X3DAudio1_7.dll" "C:/Users/arma3/Desktop/temp/X3DAudio1_7.dll"
COPY "./XAPOFX1_5.dll" "C:/Users/arma3/Desktop/temp/XAPOFX1_5.dll"

COPY "./vcredist_x64.exe" "C:/Users/arma3/Desktop/temp/vcredist_x64.exe"

RUN icacls "C:/Users/arma3/Desktop" /grant arma3:(OI)(CI)F /t

RUN powershell.exe -Command Start-Process "C:/Users/arma3/Desktop/temp/vcredist_x64.exe" -ArgumentList '/quiet' -Wait

USER arma3

WORKDIR "C:/Users/arma3/Desktop/arma3server"
VOLUME "C:/Users/arma3/Desktop/arma3server"

EXPOSE 2302/udp
EXPOSE 2303/udp
EXPOSE 2304/udp
EXPOSE 2305/udp
EXPOSE 2306/udp

ENTRYPOINT ["powershell", "-Command", "C:/Users/arma3/Desktop/entrypoint.ps1"]