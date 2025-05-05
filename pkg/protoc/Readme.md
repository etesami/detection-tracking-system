# Proto Readme

Install https://github.com/protocolbuffers/protobuf/releases

```bash

go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest


protoc \
  --go_out=. \
  --go_opt=paths=source_relative \
  --go-grpc_out=. \
  --go-grpc_opt=paths=source_relative detection_tracking_pipeline.proto.proto


pip install grpcio
pip install grpcio-tools

python -m grpc_tools.protoc \
  --python_out=. \
  --grpc_python_out=. \
  -I. detection_tracking_pipeline.proto.proto

  
```