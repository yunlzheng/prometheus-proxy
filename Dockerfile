FROM gliderlabs/alpine:3.4
MAINTAINER yunlzheng

EXPOSE 9174

RUN addgroup proxy \
 && adduser -S -G proxy proxy

COPY . /go/src/github.com/yunlzheng/prometheus-proxy

 RUN apk --update add ca-certificates jq curl\
  && apk --update add --virtual build-deps go git \
  && cd /go/src/github.com/yunlzheng/prometheus-proxy \
  && GOPATH=/go \
  && GOPATH=/go go build -o /bin/prometheus_proxy \
  && apk del --purge build-deps \
  && rm -rf /go/bin /go/pkg /var/cache/apk/*

USER proxy

ENTRYPOINT [ "/bin/prometheus_proxy" ]
