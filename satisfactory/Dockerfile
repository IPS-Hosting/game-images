FROM ubuntu:22.04@sha256:104ae83764a5119017b8e8d6218fa0832b09df65aae7d5a6de29a85d813da2fb

RUN export DEBIAN_FRONTEND=noninteractive \
    && groupadd -g 1000 ips-hosting \
    && useradd -m -d /home/ips-hosting -u 1000 -g 1000 ips-hosting \
    && apt-get update \
    && apt-get install -y lib32gcc-s1 wget tar tzdata libsdl2-dev \
    && apt-get clean \
    && rm -rf /var/lib/apt/* /tmp/* /var/tmp/*

COPY --chown=ips-hosting:ips-hosting --chmod=777 ./entrypoint.sh /ips-hosting/

USER ips-hosting
WORKDIR /home/ips-hosting
VOLUME /home/ips-hosting

ENV HOME=/home/ips-hosting

EXPOSE 7777/udp
EXPOSE 7777/tcp
EXPOSE 8888/tcp

ENTRYPOINT ["/bin/bash", "/ips-hosting/entrypoint.sh"]
