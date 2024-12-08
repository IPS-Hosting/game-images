FROM ubuntu:24.04

RUN export DEBIAN_FRONTEND=noninteractive \
    && groupadd -g 1001 ips-hosting \
    && useradd -m -d /home/ips-hosting -u 1001 -g 1001 ips-hosting \
    && dpkg --add-architecture 'i386' \
    && apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates lib32stdc++6 lib32gcc-s1 wget tar tzdata libsdl2-dev wine wine64 wine32 xvfb winbind \
    && apt-get clean \
    && rm -rf /var/lib/apt/* /tmp/* /var/tmp/*

COPY --chown=ips-hosting:ips-hosting --chmod=777 ./entrypoint.sh /ips-hosting/

USER ips-hosting
WORKDIR /home/ips-hosting
VOLUME /home/ips-hosting

ENV HOME=/home/ips-hosting
ENV WINEPREFIX=/home/ips-hosting/.wine
ENV WINEARCH=win64

# The public branch of sbox is not updated and does not launch, because it requires dotnet 7 instead of 9.
ENV BETA_BRANCH=staging

EXPOSE 27015/tcp

ENTRYPOINT ["/bin/bash", "/ips-hosting/entrypoint.sh"]
