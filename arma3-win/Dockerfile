FROM mcr.microsoft.com/windows/servercore:ltsc2022

RUN NET USER /add arma3

USER arma3

COPY "./entrypoint.ps1" "C:/Users/arma3/Desktop/entrypoint.ps1"
 
COPY "./X3DAudio1_7.dll" "C:/Users/arma3/Desktop/temp/X3DAudio1_7.dll"
COPY "./XAPOFX1_5.dll" "C:/Users/arma3/Desktop/temp/XAPOFX1_5.dll"

COPY "./vcredist_x64.exe" "C:/Users/arma3/Desktop/temp/vcredist_x64.exe"

WORKDIR "C:/Users/arma3/Desktop/arma3server"
VOLUME "C:/Users/arma3/Desktop/arma3server"

EXPOSE 2302/udp
EXPOSE 2303/udp
EXPOSE 2304/udp
EXPOSE 2305/udp
EXPOSE 2306/udp

RUN powershell.exe -Command Start-Process "C:/Users/arma3/Desktop/temp/vcredist_x64.exe" -ArgumentList '/quiet' -Wait

ENTRYPOINT ["powershell", "-Command", "C:/Users/arma3/Desktop/entrypoint.ps1"]