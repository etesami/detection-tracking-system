#!/bin/bash


# function
build() {
  local SVC_NAME=$1
  local TAG=$2
  cd $DIR/$SVC_NAME && \
    sudo docker build . -t $SVC_NAME:$TAG -t registry.skycluster.io/$SVC_NAME:$TAG
}

upload() {
  local SVC_NAME=$1
  local TAG=$2

  image_id=$(sudo docker image ls | grep $SVC_NAME | grep $TAG | awk '{print $3}' | head -n 1)

  docker_upload $image_id $SVC_NAME:$TAG scinet && \
    docker_upload $image_id $SVC_NAME:$TAG vaughan
}

DIR=$(dirname "$(pwd)")

TAG=0.0.3

SVC_NAME=svc-aggregator
build $SVC_NAME $TAG
upload $SVC_NAME $TAG

SVC_NAME=svc-detector
build $SVC_NAME $TAG
upload $SVC_NAME $TAG

# This require ffmpeg, esn-k8s-2
# SVC_NAME=svc-rtsp-server
# build $SVC_NAME $TAG
# upload $SVC_NAME $TAG

SVC_NAME=svc-tracker
build $SVC_NAME $TAG
upload $SVC_NAME $TAG

