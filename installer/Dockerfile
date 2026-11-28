# ipshosting/installer: runs the steamcmd and script install stages of game definitions (gamedef
# SPEC §3.2). Contract: steamcmd, curl, jq, unzip, tar, xz, $STEAMCMD_DIR (bootstrapped steamcmd
# incl. linux32/linux64 steamclient.so), runs as non-root, works on a read-only root filesystem.
#
#   podman build -t ipshosting/installer:v1 installer/
#   DOCKER_BUILDKIT=0 docker build -t ipshosting/installer:v1 installer/
#
# Do not build with BuildKit: there the 2017 steamcmd bootstrapper of the tarball fails every
# download ("http error 0", also with --network host), so the self-update never happens. In a
# normal container and with podman/buildah it works.
FROM debian:trixie-slim@sha256:8aa5a7bf5a1955bddbe7d74fcdd0b91ab30e74362a8b73af672b0481b35b44a0

RUN export DEBIAN_FRONTEND=noninteractive \
	&& apt-get update \
	&& apt-get install -y --no-install-recommends \
		ca-certificates curl jq unzip tar gzip xz-utils bzip2 tzdata procps \
		lib32gcc-s1 lib32stdc++6 \
	&& rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* \
	&& groupadd -g 1000 ips-installer \
	&& useradd -m -u 1000 -g 1000 -d /home/ips-installer ips-installer \
	&& install -d -o 1000 -g 1000 -m 0755 /opt/steamcmd

ENV STEAMCMD_DIR=/opt/steamcmd

USER 1000:1000

# Bootstrap steamcmd at build time (the first run updates itself). The first run may exit non-zero
# after the self-update, so success is judged by the client libraries.
RUN curl -fsSL https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz | tar -xz -C /opt/steamcmd \
	&& (HOME=/tmp/bootstrap /opt/steamcmd/steamcmd.sh +quit || HOME=/tmp/bootstrap /opt/steamcmd/steamcmd.sh +quit || true) \
	&& test -f /opt/steamcmd/linux64/steamclient.so \
	&& test -f /opt/steamcmd/linux32/steamclient.so \
	&& rm -rf /tmp/bootstrap /opt/steamcmd/logs/* /opt/steamcmd/appcache/httpcache \
	&& chmod -R a+rX /opt/steamcmd

USER root
COPY steamcmd /usr/local/bin/steamcmd
RUN chmod 0755 /usr/local/bin/steamcmd
USER 1000:1000
WORKDIR /home/ips-installer
