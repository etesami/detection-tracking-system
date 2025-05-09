# docker buildx build -t registry.skycluster.io/svc-rtsp-server:0.0.7 --platform=linux/amd64,linux/arm64 --push .
# Once build upload to registry

# sudo docker run \
#   -v /tmp/videos:/root/videos \
#   -p 8001:8001 \
#   -p 8554:50052 \
#   --rm \
#   --pull always \
#   --env-file .env \
#   --env-file .env_histogram \
#   registry.skycluster.io/svc-rtsp-server:0.0.5


###################
# multiarch build stage
###################

FROM registry.skycluster.io/ubuntu-24.04-ffmpeg:0.0.2 AS build-final

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /root/rtsp-server cmd/main.go

WORKDIR /root/

EXPOSE 8554 8001
CMD ["/root/rtsp-server"]