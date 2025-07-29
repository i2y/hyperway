#!/bin/bash

echo "=== Hyperway Protocol Tests ==="

# 1. Connect + JSON
echo -e "\n1. Connect + JSON Protocol:"
curl -X POST \
  -H "Content-Type: application/json" \
  -H "Connect-Protocol-Version: 1" \
  -d '{"name":"Connect JSON Test"}' \
  http://localhost:8080/grpcweb.example.v1.GreeterService/Greet | jq .

# 2. Connect + Protobuf (using buf)
echo -e "\n2. Connect + Protobuf Protocol:"
buf curl \
  --protocol connect \
  --schema grpc-real-comparison/greeter.proto \
  --data '{"name":"Connect Proto Test"}' \
  http://localhost:8080/grpcweb.example.v1.GreeterService/Greet

# 3. gRPC protocol (using buf)
echo -e "\n3. gRPC Protocol:"
buf curl \
  --protocol grpc \
  --schema grpc-real-comparison/greeter.proto \
  --data '{"name":"gRPC Test"}' \
  --http2-prior-knowledge \
  http://localhost:8080/grpcweb.example.v1.GreeterService/Greet

# 4. gRPC-Web protocol
echo -e "\n4. gRPC-Web Protocol:"
buf curl \
  --protocol grpcweb \
  --schema grpc-real-comparison/greeter.proto \
  --data '{"name":"gRPC-Web Test"}' \
  http://localhost:8080/grpcweb.example.v1.GreeterService/Greet

# 5. REST/JSON (if supported)
echo -e "\n5. REST/JSON:"
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"name":"REST Test"}' \
  http://localhost:8080/grpcweb.example.v1.GreeterService/Greet | jq .

# Calculate method tests
echo -e "\n=== Calculate Method Tests ==="

echo -e "\n6. Calculate with Connect + JSON:"
curl -X POST \
  -H "Content-Type: application/json" \
  -H "Connect-Protocol-Version: 1" \
  -d '{"a":100,"b":50,"operator":"/"}' \
  http://localhost:8080/grpcweb.example.v1.GreeterService/Calculate | jq .

echo -e "\n7. Calculate with gRPC:"
buf curl \
  --protocol grpc \
  --schema grpc-real-comparison/greeter.proto \
  --data '{"a":15,"b":3,"operator":"*"}' \
  --http2-prior-knowledge \
  http://localhost:8080/grpcweb.example.v1.GreeterService/Calculate
