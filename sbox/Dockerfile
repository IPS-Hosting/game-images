FROM ubuntu:24.04

RUN export DEBIAN_FRONTEND=noninteractive \
	&& groupadd -g 1001 ips-hosting \
	&& useradd -m -d /home/ips-hosting -u 1001 -g 1001 ips-hosting \
	&& apt-get update \
	&& apt-get install -y lib32stdc++6 lib32gcc-s1 wget tar tzdata libsdl2-dev \
	&& apt-get clean \
	&& rm -rf /var/lib/apt/* /tmp/* /var/tmp/*

COPY --chown=ips-hosting:ips-hosting --chmod=777 ./entrypoint.sh /ips-hosting/

USER ips-hosting
WORKDIR /home/ips-hosting
VOLUME /home/ips-hosting

ENV HOME=/home/ips-hosting

ENTRYPOINT ["/bin/bash", "/ips-hosting/entrypoint.sh"]
