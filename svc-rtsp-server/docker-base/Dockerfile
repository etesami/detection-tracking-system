# docker buildx build -f Dockerfile -t registry.skycluster.io/ubuntu-24.04-ffmpeg:0.0.2 --platform=linux/amd64,linux/arm64 --push .

FROM ubuntu:24.04 AS builder-base

ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -yq ffmpeg tar curl vim wget ca-certificates \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

FROM builder-base  AS build-base
ARG TARGETARCH

# Install GO
RUN wget -P /tmp https://go.dev/dl/go1.23.8.linux-${TARGETARCH}.tar.gz && \
    tar -C /usr/local -xzf /tmp/go1.23.8.linux-${TARGETARCH}.tar.gz && \
    rm -rf /tmp/go1.23.8.linux-${TARGETARCH}.tar.gz

ENV GOPATH=/go
ENV PATH=$GOPATH/bin:/usr/local/go/bin:$PATH
RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R 777 "$GOPATH"

CMD ["go", "version"]
    