#!/bin/bash

HELP="
Usage: build-upload.sh [OPTIONS]
  -b build
  -u upload
"

while [[ "$#" -gt 0 ]]; do
  case $1 in
    -b|--build) build=1 ;;
    -u|--upload) upload=1 ;;
    *) echo $HELP; exit 1 ;;
  esac
  shift
done

source $HOME/.functions.sh

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

SERVICES=("svc-aggregator" "svc-detector" "svc-tracker")

for SVC_NAME in "${SERVICES[@]}"; do
  [[ $build -eq 1 ]] && build $SVC_NAME $TAG
  [[ $upload -eq 1 ]] && upload $SVC_NAME $TAG
done


# This require ffmpeg, esn-k8s-2
# SVC_NAME=svc-rtsp-server
# if [[ $build -eq 1 ]]; then
#   build $SVC_NAME $TAG
# fi
# if [[ $upload -eq 1 ]]; then
#   upload $SVC_NAME $TAG
# fi
