ARG BASE_IMAGE=mini-rpki:local
FROM ${BASE_IMAGE}
USER root
ENV MINIRPKI_SOCKET=/run/minirpki/control.sock
RUN apk add --no-cache iproute2 su-exec
COPY start.sh /clab-start.sh
ENTRYPOINT ["sh", "/clab-start.sh"]
CMD []
