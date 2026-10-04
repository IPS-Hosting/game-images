FROM ubuntu:26.04@sha256:f144425ff09be612d6d9ad965196e9cdc23dae1f42110a8a11a3e9a8198759f7

RUN export DEBIAN_FRONTEND=noninteractive \
	&& groupadd -g 4711 ips-hosting \
	&& useradd -m -d /home/ips-hosting -u 4711 -g 4711 ips-hosting \
	&& dpkg --add-architecture 'i386' \
	&& apt-get update \
	&& apt-get install -y lib32stdc++6 lib32gcc-s1 libtinfo6 libtinfo6:i386 curl gdb wget tar rsync tzdata libsdl2-dev \
	&& apt-get clean \
	&& rm -rf /var/lib/apt/* /tmp/* /var/tmp/*

COPY --chown=ips-hosting:ips-hosting --chmod=777 ./entrypoint.sh ./mount.cfg /ips-hosting/

USER ips-hosting
WORKDIR /home/ips-hosting
VOLUME /home/ips-hosting

ENV HOME=/home/ips-hosting

EXPOSE 27015/udp
EXPOSE 27015/tcp
EXPOSE 27005/tcp

ENTRYPOINT ["/bin/bash", "/ips-hosting/entrypoint.sh"]
