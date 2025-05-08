# sudo docker build . -t svc-detector:0.0.3 -t registry.skycluster.io/svc-detector:0.0.3

# sudo docker run \
#   -v /tmp/videos:/root/videos \
#   -v /home/ehsan/detection-tracking-system/svc-detector/:/root/svc-detector \
#   -p 8003:8001 \
#   -p 50053:50052 \
#   --pull always \
#   --env-file .env \
#   -ti \
#   registry.skycluster.io/svc-detector:0.0.1 /bin/bash 

FROM registry.skycluster.io/opencv:4.11.0-custom AS build

WORKDIR /app

COPY go.mod go.sum ./
COPY other/shared/yolov8n-amd64.onnx /root/yolov8n.onnx
RUN go mod download

COPY . .

RUN go build -o /root/detector cmd/main.go

WORKDIR /root/

EXPOSE 50052 8001
CMD ["/root/detector"]