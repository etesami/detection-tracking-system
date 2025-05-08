# sudo docker build -t svc-detector-model-py .
# sudo docker run -v ./shared:/root -e YOLO_CONFIG_DIR="/tmp" --rm svc-detector-model-py 


# Use the Ultralytics image as base
FROM ultralytics/ultralytics:latest-cpu

# Create a directory for models inside the container
RUN mkdir -p /models

# Set the working directory
WORKDIR /root

# Define the default command (expects model path via environment or CLI arg)
CMD ["python3", "main.py"]

