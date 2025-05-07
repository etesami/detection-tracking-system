#!/bin/bash
# export UPDATE_FREQUENCY=5

export SVC_DETECTOR_HOST=localhost
export SVC_DETECTOR_PORT=5003

export REMOTE_TRACKER_HOST=localhost
export REMOTE_TRACKER_PORT=5004

export METRIC_ADDR=localhost
export METRIC_PORT=8003

export YOLO_MODEL="/home/ehsan/detection-tracking-system/svc-detector/other/yolov8n.onnx"
export IMAGE_WIDTH=640
export IMAGE_HEIGHT=640

export SAVE_IMAGE="true"
export SAVE_IMAGE_PATH="/tmp/imgs/"
export SAVE_IMAGE_FREQUENCY=1