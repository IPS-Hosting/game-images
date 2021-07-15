FROM ubuntu:20.04

RUN apt-get update \
	&& groupadd -g 1000 ips-hosting \
	&& useradd -m -d /home/ips-hosting -u 1000 -g 1000 ips-hosting \
	&& apt-get install -y lib32stdc++6 lib32gcc1 wget tar net-tools rename \
	&& apt-get clean \
	&& rm -rf /var/lib/apt/* /tmp/* /var/tmp/*

USER ips-hosting
WORKDIR /home/ips-hosting

COPY --chown=ips-hosting:ips-hosting --chmod=777 ./entrypoint.sh /ips-hosting/

USER ips-hosting
WORKDIR /home/ips-hosting
VOLUME /home/ips-hosting

EXPOSE 2302/udp
EXPOSE 2303/udp
EXPOSE 2304/udp
EXPOSE 2305/udp
EXPOSE 2306/udp


ENTRYPOINT ["/bin/bash", "/ips-hosting/entrypoint.sh"]
