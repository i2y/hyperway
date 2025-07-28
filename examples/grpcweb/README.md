# gRPC-Web Example

This example demonstrates how to use Hyperway's gRPC-Web support to enable browser-based clients to communicate with your services.

## Features

- Full gRPC-Web protocol support (both binary and base64 modes)
- Browser-based client without any proxy requirements
- Automatic protocol detection and routing
- CORS support for browser access
- Example services: Greeter and Calculator

## Running the Example

1. Start the server:
```bash
go run main.go
```

2. Open your browser and navigate to:
```
http://localhost:8080
```

3. Try the interactive demo:
   - Enter your name in the Greeter service
   - Perform calculations with the Calculator service

## How It Works

### Server Side

The server uses Hyperway's gateway to automatically handle multiple protocols:
- Standard gRPC (HTTP/2)
- gRPC-Web (HTTP/1.1 with special framing)
- Connect RPC (if configured)

All protocols are served on the same port, and the gateway automatically detects and routes requests based on headers.

### Client Side

The HTML page includes a simple gRPC-Web client implementation that:
1. Encodes requests using gRPC-Web framing (5-byte header + payload)
2. Uses base64 encoding for browser compatibility
3. Sends requests with appropriate headers (`application/grpc-web-text`)
4. Decodes responses and extracts both data and trailers

### Message Format

gRPC-Web uses a specific wire format:
- **Header**: 1 byte flag (0x00 for data, 0x80 for trailers) + 4 bytes length (big-endian)
- **Payload**: The actual message data (protobuf in production, JSON in this demo)
- **Trailers**: HTTP/1.1 style headers sent as a final frame with flag 0x80

## Protocol Details

### Request Headers
```
Content-Type: application/grpc-web-text
X-Grpc-Web: 1
```

### Response Format
1. Data frame(s) containing the response message
2. Trailer frame containing gRPC status and metadata

### Status Codes
The gRPC status is returned in the trailer frame:
- `grpc-status: 0` - OK
- `grpc-status: 5` - NOT_FOUND
- etc. (standard gRPC status codes)

## Testing with curl

You can also test the gRPC-Web endpoint using curl:

```bash
# Create a simple request frame (this example uses a simplified format)
# In production, you'd use proper protobuf encoding

# For base64 mode:
echo -n '{"name":"Test"}' | \
  awk '{printf "\x00\x00\x00\x00%c%s", length($0), $0}' | \
  base64 | \
  curl -X POST http://localhost:8080/grpcweb.example.v1.GreeterService/Greet \
    -H "Content-Type: application/grpc-web-text" \
    -H "X-Grpc-Web: 1" \
    --data-binary @-
```

## Production Considerations

1. **Protobuf Encoding**: This demo uses JSON for simplicity. In production, use proper protobuf encoding.
2. **Authentication**: Add authentication headers as needed.
3. **Streaming**: Hyperway's current implementation supports unary RPCs. Streaming support can be added.
4. **Error Handling**: Implement comprehensive error handling and status code mapping.
5. **Client Libraries**: Consider using official gRPC-Web client libraries for production applications.

## Supported Browsers

gRPC-Web works in all modern browsers that support:
- Fetch API
- ArrayBuffer and Uint8Array
- Base64 encoding/decoding

This includes:
- Chrome/Edge 42+
- Firefox 39+
- Safari 10.1+

## Differences from Standard gRPC

1. **Transport**: Uses HTTP/1.1 instead of HTTP/2
2. **Streaming**: Limited streaming support (server streaming only in most implementations)
3. **Trailers**: Sent as a message frame instead of HTTP/2 trailers
4. **Binary Mode**: Some proxies/browsers may have issues with binary mode, so base64 is often preferred