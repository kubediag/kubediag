FROM --platform=${TARGETPLATFORM:-linux/amd64} ghcr.io/openfaas/of-watchdog:0.9.10 as watchdog
FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.18-alpine3.16 as build

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

RUN apk --no-cache add git

COPY --from=watchdog /fwatchdog /usr/bin/fwatchdog
RUN chmod +x /usr/bin/fwatchdog


RUN mkdir -p /go/src/handler
WORKDIR /go/src/handler
COPY . .

ARG GO111MODULE="on"
ARG GOPROXY=""
ARG GOFLAGS=""
ARG CGO_ENABLED=0
ENV CGO_ENABLED=${CGO_ENABLED}

RUN sh ./modules-cleanup.sh

WORKDIR /go/src/handler/function

RUN mkdir -p /go/src/handler/function/static

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOFLAGS=${GOFLAGS} go test ./... -cover

WORKDIR /go/src/handler
RUN CGO_ENABLED=${CGO_ENABLED} GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOFLAGS=${GOFLAGS} \
    go build --ldflags "-s -w" -a -installsuffix cgo -o handler .

FROM --platform=${TARGETPLATFORM:-linux/amd64} alpine:3.16.2 as ship
# Add non root user and certs
RUN apk --no-cache add ca-certificates \
    && addgroup -S app && adduser -S -g app app
# Split instructions so that buildkit can run & cache
# the previous command ahead of time.
RUN mkdir -p /home/app \
    && chown app /home/app

WORKDIR /home/app

COPY --from=build --chown=app /go/src/handler/handler           .
COPY --from=build --chown=app /usr/bin/fwatchdog                .
COPY --from=build --chown=app /go/src/handler/function/static   static

USER app

ENV fprocess="./handler"
ENV mode="http"
ENV upstream_url="http://127.0.0.1:8082"
ENV prefix_logs="false"

CMD ["./fwatchdog"]
