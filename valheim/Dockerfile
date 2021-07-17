FROM mono:6.12

RUN DEBIAN_FRONTEND=noninteractive \
	&& groupadd -g 1000 ips-hosting \
	&& useradd -m -d /home/ips-hosting -u 1000 -g 1000 ips-hosting \
	&& apt-get update \
	&& apt-get install -y lib32stdc++6 lib32gcc1 libc6-dev wget tar \
	&& apt-get clean \
	&& rm -rf /var/lib/apt/* /tmp/* /var/tmp/*

COPY --chown=ips-hosting:ips-hosting --chmod=777 ./entrypoint.sh /ips-hosting/

USER ips-hosting
WORKDIR /home/ips-hosting
VOLUME /home/ips-hosting

EXPOSE 2456/udp
EXPOSE 2457/udp

ENTRYPOINT ["/bin/bash", "/ips-hosting/entrypoint.sh"]
