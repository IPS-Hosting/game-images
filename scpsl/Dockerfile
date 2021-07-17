FROM mono:6.12

# libGDI+ is required by some popular plugins like https://umod.org/plugins/sign-artist
RUN DEBIAN_FRONTEND=noninteractive \
	&& groupadd -g 1000 ips-hosting \
	&& useradd -m -d /home/ips-hosting -u 1000 -g 1000 ips-hosting \
	&& apt-get update \
	&& apt-get install -y lib32stdc++6 lib32gcc1 wget tar libgdiplus \
	&& apt-get clean \
	&& rm -rf /var/lib/apt/* /tmp/* /var/tmp/*

COPY --chown=ips-hosting:ips-hosting --chmod=777 ./entrypoint.sh /ips-hosting/

USER ips-hosting
WORKDIR /home/ips-hosting
VOLUME /home/ips-hosting

EXPOSE 7777/udp
EXPOSE 7777/tcp

ENTRYPOINT ["/bin/bash", "/ips-hosting/entrypoint.sh"]
