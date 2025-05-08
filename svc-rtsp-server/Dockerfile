# sudo docker build . -t svc-rtsp-server -t registry.skycluster.io/svc-rtsp-server:0.0.1
# Once build upload to registry

# sudo docker run \
#   -v /tmp/videos:/root/videos \
#   -p 8001:8001 \
#   -p 8554:50052 \
#   --pull always \
#   --env-file .env \
#   registry.skycluster.io/svc-rtsp-server:0.0.1

FROM registry.skycluster.io/ubuntu-24-04-ffmpeg:0.0.1  AS build

RUN apt-get update && \
    apt-get install -y --no-install-recommends wget ca-certificates

# Install GO
RUN wget -P /tmp https://go.dev/dl/go1.23.8.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf /tmp/go1.23.8.linux-amd64.tar.gz && \
    rm -rf /tmp/go1.23.8.linux-amd64.tar.gz
    
ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH
RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R 777 "$GOPATH"


WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /root/rtsp-server cmd/main.go

WORKDIR /root/

EXPOSE 8554 8001
CMD ["/root/rtsp-server"]