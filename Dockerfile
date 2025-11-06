FROM golang:1.20.4-alpine AS build

LABEL maintainer="Iori Mizutani <iori.mizutani@gmail.com>"

# build the app
RUN mkdir -p /build
COPY go.mod /build/
COPY go.sum /build/
COPY *.go /build/
COPY assets/ /build/assets/
COPY internal/ /build/internal/
COPY cmd/radikron/ /build/cmd/radikron/
WORKDIR /build
RUN go mod vendor
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o radikron ./cmd/radikron/...

# export to a single layer image
FROM alpine:latest

# install some required binaries
RUN apk add --no-cache ca-certificates \
    ffmpeg \
    tzdata

WORKDIR /app

COPY --from=build /build/radikron /app/radikron

# set timezone
ENV TZ "Asia/Tokyo"
# set the default download dir
ENV RADICRON_HOME "/radiko"
VOLUME ["/radiko"]

ENTRYPOINT ["/app/radikron"]
CMD ["-c", "/app/config.yml"]
