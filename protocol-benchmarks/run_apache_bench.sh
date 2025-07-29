#!/bin/bash

# Apache Bench tests for Connect + JSON protocol
# Compares Connect-Go vs Hyperway performance

echo "Running Apache Bench tests for Connect + JSON protocol..."
echo "=============================================="

# Configuration
REQUESTS=100000
CONCURRENCY=100
CONNECT_GO_PORT=8084
HYPERWAY_PORT=8080
ENDPOINT="/grpcweb.example.v1.GreeterService/Greet"
PAYLOAD='{"name":"Test"}'

# Ensure servers are running
echo "Make sure both servers are running:"
echo "- Connect-Go server on port $CONNECT_GO_PORT"
echo "- Hyperway server on port $HYPERWAY_PORT"
echo ""
read -p "Press Enter to continue..."

# Test Connect-Go
echo ""
echo "Testing Connect-Go (port $CONNECT_GO_PORT)..."
echo "----------------------------------------------"
ab -n $REQUESTS -c $CONCURRENCY -k \
   -p /dev/stdin \
   -T "application/json" \
   http://127.0.0.1:${CONNECT_GO_PORT}${ENDPOINT} <<< "$PAYLOAD"

# Small pause between tests
sleep 2

# Test Hyperway
echo ""
echo "Testing Hyperway (port $HYPERWAY_PORT)..."
echo "----------------------------------------------"
ab -n $REQUESTS -c $CONCURRENCY -k \
   -p /dev/stdin \
   -T "application/json" \
   http://127.0.0.1:${HYPERWAY_PORT}${ENDPOINT} <<< "$PAYLOAD"

echo ""
echo "Tests completed! Check the 'Requests per second' metric for comparison."
