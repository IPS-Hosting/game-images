FROM mono:6.12

# libGDI+ is required by some popular plugins like https://umod.org/plugins/sign-artist
RUN export DEBIAN_FRONTEND=noninteractive \
	&& groupadd -g 1000 ips-hosting \
	&& useradd -m -d /home/ips-hosting -u 1000 -g 1000 ips-hosting \
	&& apt-get update \
	&& apt-get install -y lib32stdc++6 lib32gcc1 wget tar libgdiplus tzdata libsdl2-dev libc6-dev \
	&& apt-get clean \
	&& rm -rf /var/lib/apt/* /tmp/* /var/tmp/*

COPY --chown=ips-hosting:ips-hosting --chmod=777 ./entrypoint.sh /ips-hosting/

USER ips-hosting
WORKDIR /home/ips-hosting
VOLUME /home/ips-hosting

# https://github.com/northwood-studios/LocalAdmin-V2/issues/18#issuecomment-758485918
ENV HOME=/home/ips-hosting

EXPOSE 7777/udp
EXPOSE 7777/tcp

ENTRYPOINT ["/bin/bash", "/ips-hosting/entrypoint.sh"]
