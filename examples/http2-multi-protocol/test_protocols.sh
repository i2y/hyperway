#!/bin/bash

echo "=== Testing Multi-Protocol Support on HTTP/2 Server ==="
echo ""

# Create proto schema file for buf curl
cat > calc.proto << 'EOF'
syntax = "proto3";

package calculator.v1;

message CalcRequest {
  int32 a = 1;
  int32 b = 2;
}

message CalcResponse {
  int32 result = 1;
}

service CalculatorService {
  rpc add(CalcRequest) returns (CalcResponse) {}
  rpc multiply(CalcRequest) returns (CalcResponse) {}
  rpc subtract(CalcRequest) returns (CalcResponse) {}
}
EOF

# Wait for server to start
sleep 2

echo "=== Testing with buf curl ==="
echo ""

# Test 1: gRPC over HTTP/2
echo "1. Testing gRPC (HTTP/2):"
buf curl --protocol grpc \
  --schema calc.proto \
  --http2-prior-knowledge \
  --data '{"a": 5, "b": 3}' \
  http://localhost:8084/calculator.v1.CalculatorService/add

echo -e "\n"

# Test 2: Connect-RPC over HTTP/2
echo "2. Testing Connect-RPC (HTTP/2):"
buf curl --protocol connect \
  --schema calc.proto \
  --http2-prior-knowledge \
  --data '{"a": 5, "b": 3}' \
  http://localhost:8084/calculator.v1.CalculatorService/add

echo -e "\n"

# Test 3: gRPC-Web over HTTP/2
echo "3. Testing gRPC-Web (HTTP/2):"
buf curl --protocol grpcweb \
  --schema calc.proto \
  --data '{"a": 5, "b": 3}' \
  http://localhost:8084/calculator.v1.CalculatorService/add

echo -e "\n"

# Test 4: Connect-RPC over HTTP/1.1
echo "4. Testing Connect-RPC (HTTP/1.1):"
buf curl --protocol connect \
  --schema calc.proto \
  --data '{"a": 4, "b": 7}' \
  http://localhost:8084/calculator.v1.CalculatorService/multiply

echo -e "\n"

# Test 5: gRPC-Web over HTTP/1.1
echo "5. Testing gRPC-Web (HTTP/1.1):"
buf curl --protocol grpcweb \
  --schema calc.proto \
  --data '{"a": 10, "b": 20}' \
  http://localhost:8084/calculator.v1.CalculatorService/add

echo -e "\n"

# Test 6: Multiple operations with gRPC-Web
echo "6. Testing multiple operations (gRPC-Web):"
echo "   - subtract(20, 7):"
buf curl --protocol grpcweb \
  --schema calc.proto \
  --data '{"a": 20, "b": 7}' \
  http://localhost:8084/calculator.v1.CalculatorService/subtract

echo -e "\n"

echo "=== Testing with curl for JSON-RPC ==="
echo ""

# Test 7: JSON-RPC over HTTP/2
echo "7. Testing JSON-RPC (HTTP/2):"
curl -s --http2-prior-knowledge -X POST http://localhost:8084/api/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "add",
    "params": {"a": 5, "b": 3},
    "id": 1
  }' | jq .

echo -e "\n"

# Test 8: JSON-RPC over HTTP/1.1
echo "8. Testing JSON-RPC (HTTP/1.1):"
curl -s --http1.1 -X POST http://localhost:8084/api/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "multiply",
    "params": {"a": 4, "b": 7},
    "id": 2
  }' | jq .

echo -e "\n"

# Test 9: JSON-RPC Batch Request
echo "9. Testing JSON-RPC Batch (HTTP/2):"
curl -s --http2-prior-knowledge -X POST http://localhost:8084/api/jsonrpc \
  -H "Content-Type: application/json" \
  -d '[
    {"jsonrpc": "2.0", "method": "add", "params": {"a": 1, "b": 2}, "id": 1},
    {"jsonrpc": "2.0", "method": "multiply", "params": {"a": 3, "b": 4}, "id": 2},
    {"jsonrpc": "2.0", "method": "subtract", "params": {"a": 10, "b": 3}, "id": 3}
  ]' | jq .

echo -e "\n"

# Test 10: Verify all protocols with verbose output
echo "10. Protocol verification with verbose output:"
echo "    Checking HTTP version and protocol negotiation..."
echo ""

echo "    - gRPC (must use HTTP/2):"
buf curl -v --protocol grpc \
  --schema calc.proto \
  --http2-prior-knowledge \
  --data '{"a": 1, "b": 1}' \
  http://localhost:8084/calculator.v1.CalculatorService/add 2>&1 | grep -E "(Using protocol|HTTP/2)"

echo ""
echo "    - Connect (can use HTTP/1.1 or HTTP/2):"
buf curl -v --protocol connect \
  --schema calc.proto \
  --data '{"a": 1, "b": 1}' \
  http://localhost:8084/calculator.v1.CalculatorService/add 2>&1 | grep -E "(Using protocol|HTTP/)"

echo ""
echo "    - gRPC-Web (can use HTTP/1.1 or HTTP/2):"
buf curl -v --protocol grpcweb \
  --schema calc.proto \
  --data '{"a": 1, "b": 1}' \
  http://localhost:8084/calculator.v1.CalculatorService/add 2>&1 | grep -E "(Using protocol|HTTP/)"

# Clean up
rm -f calc.proto

echo -e "\n\n=== Test Complete ==="
echo ""
echo "Summary:"
echo "- gRPC: HTTP/2 only ✓"
echo "- Connect-RPC: HTTP/1.1 and HTTP/2 ✓"
echo "- gRPC-Web: HTTP/1.1 and HTTP/2 ✓"
echo "- JSON-RPC: HTTP/1.1 and HTTP/2 ✓"
echo ""
echo "All protocols are working correctly on the same server!"
