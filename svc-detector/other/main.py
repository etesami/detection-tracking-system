from ultralytics import YOLO

model = YOLO("yolov8n.pt") 
model.export(format="onnx")

# model.export(format="onnx", imgsz=[360,640])