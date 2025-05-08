# sudo docker build . -t svc-tracker:0.0.1 -t registry.skycluster.io/svc-tracker:0.0.1

# sudo docker run \
#   -v /tmp/videos:/root/videos \
#   -v /home/ehsan/detection-tracking-system/svc-tracker/:/root/svc-tracker \
#   -p 8003:8001 \
#   -p 50053:50052 \
#   --pull always \
#   --env-file .env \
#   -ti \
#   registry.skycluster.io/svc-tracker:0.0.1 /bin/bash 

FROM registry.skycluster.io/opencv:4.11.0-custom AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /root/tracker cmd/main.go

WORKDIR /root/

EXPOSE 50052 8001
CMD ["/root/tracker"]