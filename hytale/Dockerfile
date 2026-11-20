FROM eclipse-temurin:25@sha256:ce2ef9d2d67a68de5e196d4de9d743327c44a10b57fe02ec39f572c652cacfd3 AS jre-build

# Create a custom Java runtime
RUN $JAVA_HOME/bin/jlink \
    --module-path $JAVA_HOME/jmods \
    --add-modules java.se,jdk.management,jdk.net,jdk.crypto.ec,jdk.unsupported,jdk.zipfs,jdk.localedata,jdk.charsets,jdk.security.auth,jdk.naming.dns,jdk.naming.rmi \
    --strip-debug \
    --no-man-pages \
    --no-header-files \
    --compress=2 \
    --output /javaruntime

FROM ubuntu:24.04@sha256:9c7f200c07e3d4a03e7249d2c16bf66a51120fb876aa2242cf31451412543289
ENV JAVA_HOME=/opt/java/openjdk
ENV PATH="${JAVA_HOME}/bin:${PATH}"
COPY --from=jre-build /javaruntime $JAVA_HOME

RUN export DEBIAN_FRONTEND=noninteractive \
    && groupadd -g 4711 ips-hosting \
    && useradd -m -d /home/ips-hosting -u 4711 -g 4711 ips-hosting \
    && touch /etc/machine-id \
    && chown ips-hosting:ips-hosting /etc/machine-id \
    && chmod 644 /etc/machine-id \
    && apt-get update \
    && apt-get install -y wget curl unzip \
    && apt-get clean \
    && rm -rf /var/lib/apt/* /tmp/* /var/tmp/*

COPY --chown=ips-hosting:ips-hosting --chmod=777 ./entrypoint.sh /ips-hosting/

USER ips-hosting
WORKDIR /home/ips-hosting
VOLUME /home/ips-hosting

ENV HOME=/home/ips-hosting

EXPOSE 5520/udp
EXPOSE 5523/tcp

ENTRYPOINT ["/bin/bash", "/ips-hosting/entrypoint.sh"]
