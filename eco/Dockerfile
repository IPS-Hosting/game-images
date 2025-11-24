FROM ubuntu:24.04@sha256:c35e29c9450151419d9448b0fd75374fec4fff364a27f176fb458d472dfc9e54

RUN export DEBIAN_FRONTEND=noninteractive \
	&& groupadd -g 4711 ips-hosting \
	&& useradd -m -d /home/ips-hosting -u 4711 -g 4711 ips-hosting \
	&& dpkg --add-architecture i386 \
	&& apt-get update \
	&& apt-get install -y lib32stdc++6 lib32gcc-s1 libc6:i386 libstdc++6:i386 wget tar tzdata libsdl2-dev libgdiplus apt-transport-https curl \
	&& wget https://packages.microsoft.com/config/ubuntu/24.04/packages-microsoft-prod.deb -O /tmp/packages-microsoft-prod.deb \
	&& dpkg -i /tmp/packages-microsoft-prod.deb \
	&& apt-get update \
	&& apt-get install -y aspnetcore-runtime-8.0 \
	&& apt-get clean \
	&& rm -rf /var/lib/apt/* /tmp/* /var/tmp/*

COPY --chown=ips-hosting:ips-hosting --chmod=777 ./entrypoint.sh /ips-hosting/

USER ips-hosting
WORKDIR /home/ips-hosting
VOLUME /home/ips-hosting

ENV HOME=/home/ips-hosting

EXPOSE 3000/udp
EXPOSE 3001/tcp

ENTRYPOINT ["/bin/bash", "/ips-hosting/entrypoint.sh"]
