FROM ubuntu:24.04@sha256:c35e29c9450151419d9448b0fd75374fec4fff364a27f176fb458d472dfc9e54

RUN export DEBIAN_FRONTEND=noninteractive \
	&& groupadd -g 4711 ips-hosting \
	&& useradd -m -d /home/ips-hosting -u 4711 -g 4711 ips-hosting \
	&& apt-get update \
	&& apt-get install -y ca-certificates tzdata \
	&& apt-get clean \
	&& rm -rf /var/lib/apt/* /tmp/* /var/tmp/*

USER ips-hosting
WORKDIR /home/ips-hosting
VOLUME /home/ips-hosting

ENV HOME=/home/ips-hosting
