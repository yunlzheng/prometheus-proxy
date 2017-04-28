FROM gliderlabs/alpine:3.4
MAINTAINER yunlzheng

EXPOSE 9174

RUN addgroup proxy \
 && adduser -S -G proxy proxy

RUN apk --update add ca-certificates jq curl

ADD prometheus-proxy /bin/prometheus-proxy

USER proxy

ENTRYPOINT [ "/bin/prometheus-proxy" ]
