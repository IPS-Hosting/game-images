FROM ubuntu:20.04

RUN export DEBIAN_FRONTEND=noninteractive \
	&& groupadd -g 1000 ips-hosting \
	&& useradd -m -d /home/ips-hosting -u 1000 -g 1000 ips-hosting \
	&& apt-get update \
	&& apt-get install -y lib32gcc1 wget tar curl perl-modules tzdata \
	&& apt-get clean \
	&& rm -rf /var/lib/apt/* /tmp/* /var/tmp/*
	
COPY --chown=ips-hosting:ips-hosting --chmod=777 ./entrypoint.sh /ips-hosting/

USER ips-hosting
WORKDIR /home/ips-hosting
VOLUME /home/ips-hosting

EXPOSE 27020/tcp
EXPOSE 27015/udp
EXPOSE 7777/udp
EXPOSE 7778/udp

ENTRYPOINT ["/bin/bash", "/ips-hosting/entrypoint.sh"]
