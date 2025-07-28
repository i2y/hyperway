#!/bin/bash

echo "=== Connect + Protobuf Protocol Test ==="

# Greet test
echo -e "\n1. Testing Greet method:"
curl -X POST \
  -H "Content-Type: application/proto" \
  -H "Connect-Protocol-Version: 1" \
  --data-binary $(echo -n '{"name":"Connect Proto Test"}' | protoc --encode=grpcweb.example.v1.GreetRequest greeter.proto) \
  http://localhost:8080/grpcweb.example.v1.GreeterService/Greet \
  | protoc --decode=grpcweb.example.v1.GreetResponse greeter.proto

# Calculate test
echo -e "\n\n2. Testing Calculate method:"
curl -X POST \
  -H "Content-Type: application/proto" \
  -H "Connect-Protocol-Version: 1" \
  --data-binary $(echo -n '{"a":20,"b":10,"operator":"*"}' | protoc --encode=grpcweb.example.v1.CalculateRequest greeter.proto) \
  http://localhost:8080/grpcweb.example.v1.GreeterService/Calculate \
  | protoc --decode=grpcweb.example.v1.CalculateResponse greeter.proto
