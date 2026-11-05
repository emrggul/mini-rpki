FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends frr frr-rpki-rtrlib iproute2 && rm -rf /var/lib/apt/lists/*
COPY daemons /etc/frr/daemons
RUN test -f /usr/lib/x86_64-linux-gnu/frr/modules/bgpd_rpki.so || find /usr/lib -name '*rpki*.so' | grep .
COPY start.sh /start.sh
CMD ["sh", "/start.sh"]
