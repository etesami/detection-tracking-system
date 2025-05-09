# Readme

Using the Dockerfile for `ubuntu2404-edge` from the following repository:
- https://github.com/jrottenberg/ffmpeg/blob/d21a1f25933390b931937f7d984982307fa1d0da/docker-images/7.1/ubuntu2404-edge/Dockerfile

```bash
docker buildx build -f Dockerfile -t ubuntu-24-04-ffmpeg:0.0.1 -t registry.skycluster.io/ubuntu-24-04-ffmpeg:0.0.1 --platform=linux/amd64 --load .
docker buildx build -f Dockerfile -t ubuntu-24-04-ffmpeg:0.0.1 -t registry.skycluster.io/ubuntu-24-04-ffmpeg:0.0.1 --platform=linux/arm64 --push .
```