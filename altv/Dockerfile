FROM ubuntu:26.04@sha256:c098726dd9b9733f3e885f8de0ef375206a311b0a317eadc40d347b1115fb944

RUN export DEBIAN_FRONTEND=noninteractive \
	&& groupadd -g 4711 ips-hosting \
	&& useradd -m -d /home/ips-hosting -u 4711 -g 4711 ips-hosting \
	&& apt-get update \
	&& apt-get install -y wget apt-transport-https curl jq libatomic1 tzdata \
	&& curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
	&& wget https://packages.microsoft.com/config/ubuntu/24.04/packages-microsoft-prod.deb -O packages-microsoft-prod.deb \
	&& dpkg -i packages-microsoft-prod.deb \
	&& rm packages-microsoft-prod.deb \
	&& apt-get update \
	&& apt-get install -y nodejs dotnet-sdk-8.0 \
	&& npm install --global yarn \
	&& apt-get clean && rm -rf /var/lib/apt/* /tmp/* /var/tmp/*

COPY --chown=ips-hosting:ips-hosting --chmod=777 ./entrypoint.sh /ips-hosting/

USER ips-hosting
WORKDIR /home/ips-hosting
VOLUME /home/ips-hosting

ENV HOME=/home/ips-hosting

EXPOSE 7788/udp
EXPOSE 7788/tcp

ENTRYPOINT ["/bin/bash", "/ips-hosting/entrypoint.sh"]
