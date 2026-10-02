FROM ubuntu:24.04@sha256:a853f94d226358a79c740cfc7bce0c289748f3fe3488d921d038ccd752c61b60

RUN export DEBIAN_FRONTEND=noninteractive \
	&& groupadd -g 4711 ips-hosting \
	&& useradd -m -d /home/ips-hosting -u 4711 -g 4711 ips-hosting \
	&& apt-get update \
	&& apt-get install -y lib32stdc++6 lib32gcc-s1 wget tar tzdata libsdl2-dev curl \
	&& apt-get clean \
	&& rm -rf /var/lib/apt/* /tmp/* /var/tmp/*

COPY --chown=ips-hosting:ips-hosting --chmod=777 ./entrypoint.sh /ips-hosting/

USER ips-hosting
WORKDIR /home/ips-hosting
VOLUME /home/ips-hosting

ENV HOME=/home/ips-hosting

EXPOSE 8766/udp
EXPOSE 16261/udp

ENTRYPOINT ["/bin/bash", "/ips-hosting/entrypoint.sh"]
