# sudo docker build . -t svc-aggregator -t registry.skycluster.io/svc-aggregator:0.0.1
# then upload to registry

# sudo docker run \
#   -p 8002:8001 \
#   -p 50052:50052 \
#   --pull always \
#   --env-file .env \
#   --rm \
#   registry.skycluster.io/svc-aggregator:0.0.3

FROM registry.skycluster.io/opencv:4.11.0-custom AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /root/aggregator cmd/main.go

WORKDIR /root/

EXPOSE 50052 8001
CMD ["/root/aggregator"]