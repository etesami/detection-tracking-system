# sudo docker build . -t svc-detector -t registry.skycluster.io/svc-detector:0.0.1

# sudo docker run \
#   -v /tmp/videos:/root/videos \
#   -p 8003:8001 \
#   -p 50053:50052 \
#   --pull always \
#   --env-file .env \
#   registry.skycluster.io/svc-detector:0.0.1

FROM registry.skycluster.io/opencv:4.11.0 AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /root/detector cmd/main.go

WORKDIR /root/

EXPOSE 50052 8001
CMD ["/root/detector"]