FROM ubuntu:22.04

RUN export DEBIAN_FRONTEND=noninteractive \
	&& groupadd -g 1000 ips-hosting \
	&& useradd -m -d /home/ips-hosting -u 1000 -g 1000 ips-hosting \
	&& dpkg --add-architecture 'i386' \
	&& apt-get update \
	&& apt-get install -y lib32stdc++6 lib32gcc1 libtinfo5 libtinfo5:i386 gdb wget tar rsync tzdata libsdl2-dev \
	&& apt-get clean \
	&& rm -rf /var/lib/apt/* /tmp/* /var/tmp/*

COPY --chown=ips-hosting:ips-hosting --chmod=777 ./entrypoint.sh ./mount.cfg /ips-hosting/

USER ips-hosting
WORKDIR /home/ips-hosting
VOLUME /home/ips-hosting

EXPOSE 27015/udp
EXPOSE 27015/tcp
EXPOSE 27005/tcp

ENTRYPOINT ["/bin/bash", "/ips-hosting/entrypoint.sh"]
