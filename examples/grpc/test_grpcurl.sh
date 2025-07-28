#!/bin/bash

# Test script for grpcurl compatibility

echo "Testing grpcurl compatibility with hyperway gRPC server"
echo "======================================================="
echo ""

# Check if grpcurl is installed
if ! command -v grpcurl &> /dev/null; then
    echo "grpcurl is not installed. Please install it first:"
    echo "  brew install grpcurl  # macOS"
    echo "  go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest  # Go"
    exit 1
fi

# Wait for server to be ready
echo "Waiting for server to start..."
sleep 2

# Test 1: List services
echo "1. Listing available services:"
grpcurl -plaintext localhost:8080 list
echo ""

# Test 2: Describe service
echo "2. Describing UserService:"
grpcurl -plaintext localhost:8080 describe grpc.example.v1.UserService
echo ""

# Test 3: Create a user
echo "3. Creating a new user:"
grpcurl -plaintext -d '{"name":"Alice Smith","email":"alice@example.com"}' \
    localhost:8080 grpc.example.v1.UserService/CreateUser
echo ""

# Test 4: List users
echo "4. Listing all users:"
grpcurl -plaintext -d '{"limit":10}' \
    localhost:8080 grpc.example.v1.UserService/ListUsers
echo ""

# Test 5: Get specific user
echo "5. Getting user by ID:"
grpcurl -plaintext -d '{"id":"user-0"}' \
    localhost:8080 grpc.example.v1.UserService/GetUser
echo ""

# Test 6: Create another user
echo "6. Creating another user:"
grpcurl -plaintext -d '{"name":"Bob Johnson","email":"bob@example.com"}' \
    localhost:8080 grpc.example.v1.UserService/CreateUser
echo ""

# Test 7: List users again
echo "7. Listing users again:"
grpcurl -plaintext -d '{"limit":10}' \
    localhost:8080 grpc.example.v1.UserService/ListUsers
echo ""

# Test 8: Delete a user
echo "8. Deleting a user:"
grpcurl -plaintext -d '{"id":"user-1"}' \
    localhost:8080 grpc.example.v1.UserService/DeleteUser
echo ""

# Test 9: List after deletion
echo "9. Listing users after deletion:"
grpcurl -plaintext -d '{"limit":10}' \
    localhost:8080 grpc.example.v1.UserService/ListUsers
echo ""

# Test 10: Error case - get non-existent user
echo "10. Testing error case - getting non-existent user:"
grpcurl -plaintext -d '{"id":"user-999"}' \
    localhost:8080 grpc.example.v1.UserService/GetUser
echo ""

echo "======================================================="
echo "grpcurl compatibility tests completed!"
