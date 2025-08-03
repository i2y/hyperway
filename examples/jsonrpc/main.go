package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/i2y/hyperway/rpc"
)

// Calculator service example
type AddRequest struct {
	A int `json:"a" validate:"required"`
	B int `json:"b" validate:"required"`
}

type AddResponse struct {
	Result int `json:"result"`
}

type MultiplyRequest struct {
	A int `json:"a" validate:"required"`
	B int `json:"b" validate:"required"`
}

type MultiplyResponse struct {
	Result int `json:"result"`
}

// Handler functions
func add(ctx context.Context, req *AddRequest) (*AddResponse, error) {
	return &AddResponse{
		Result: req.A + req.B,
	}, nil
}

func multiply(ctx context.Context, req *MultiplyRequest) (*MultiplyResponse, error) {
	return &MultiplyResponse{
		Result: req.A * req.B,
	}, nil
}

func subtract(ctx context.Context, req *AddRequest) (*AddResponse, error) {
	return &AddResponse{
		Result: req.A - req.B,
	}, nil
}

const defaultBatchLimit = 50

func main() {
	// Create a service with JSON-RPC enabled
	svc := rpc.NewService("CalculatorService",
		rpc.WithPackage("calculator.v1"),
		rpc.WithValidation(true),
		rpc.WithJSONRPC("/api/jsonrpc"),              // Enable JSON-RPC at /api/jsonrpc
		rpc.WithJSONRPCBatchLimit(defaultBatchLimit), // Limit batch requests
		rpc.WithDescription("A simple calculator service supporting JSON-RPC 2.0"),
	)

	// Register methods
	rpc.MustRegister(svc, "add", add)
	rpc.MustRegister(svc, "multiply", multiply)
	rpc.MustRegister(svc, "subtract", subtract)

	// Create gateway - this supports gRPC, Connect, gRPC-Web, and JSON-RPC
	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		log.Fatalf("Failed to create gateway: %v", err)
	}

	// Create HTTP server
	mux := http.NewServeMux()
	mux.Handle("/", gateway)

	// Add a simple index page
	mux.HandleFunc("/index.html", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, indexHTML)
	})

	addr := ":8082"
	log.Printf("Starting server on %s", addr)
	log.Printf("JSON-RPC endpoint: http://localhost%s/api/jsonrpc", addr)
	log.Printf("Try the demo at: http://localhost%s/index.html", addr)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
		// Security timeouts
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

const indexHTML = `<!DOCTYPE html>
<html>
<head>
    <title>JSON-RPC Calculator Demo</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .example { background: #f4f4f4; padding: 10px; margin: 10px 0; }
        button { margin: 5px; padding: 5px 10px; }
        #result { margin-top: 20px; padding: 10px; background: #e8f4f8; }
        pre { background: #f0f0f0; padding: 10px; overflow-x: auto; }
    </style>
</head>
<body>
    <h1>JSON-RPC 2.0 Calculator Demo</h1>
    
    <h2>Single Request Examples</h2>
    <button onclick="sendRequest('add', {a: 5, b: 3})">Add 5 + 3</button>
    <button onclick="sendRequest('multiply', {a: 4, b: 7})">Multiply 4 × 7</button>
    <button onclick="sendRequest('subtract', {a: 10, b: 4})">Subtract 10 - 4</button>
    
    <h2>Batch Request Example</h2>
    <button onclick="sendBatchRequest()">Send Batch Request</button>
    
    <h2>Error Example</h2>
    <button onclick="sendRequest('divide', {a: 10, b: 0})">Call Non-existent Method</button>
    
    <div id="result"></div>
    
    <script>
        async function sendRequest(method, params) {
            const request = {
                jsonrpc: "2.0",
                method: method,
                params: params,
                id: Date.now()
            };
            
            showRequest(request);
            
            try {
                const response = await fetch('/api/jsonrpc', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify(request)
                });
                
                const result = await response.json();
                showResponse(result);
            } catch (error) {
                showError(error);
            }
        }
        
        async function sendBatchRequest() {
            const requests = [
                { jsonrpc: "2.0", method: "add", params: {a: 1, b: 2}, id: 1 },
                { jsonrpc: "2.0", method: "multiply", params: {a: 3, b: 4}, id: 2 },
                { jsonrpc: "2.0", method: "subtract", params: {a: 10, b: 3}, id: 3 }
            ];
            
            showRequest(requests);
            
            try {
                const response = await fetch('/api/jsonrpc', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify(requests)
                });
                
                const results = await response.json();
                showResponse(results);
            } catch (error) {
                showError(error);
            }
        }
        
        function showRequest(request) {
            document.getElementById('result').innerHTML = 
                '<h3>Request:</h3><pre>' + JSON.stringify(request, null, 2) + '</pre>';
        }
        
        function showResponse(response) {
            document.getElementById('result').innerHTML += 
                '<h3>Response:</h3><pre>' + JSON.stringify(response, null, 2) + '</pre>';
        }
        
        function showError(error) {
            document.getElementById('result').innerHTML += 
                '<h3>Error:</h3><pre style="color: red;">' + error + '</pre>';
        }
    </script>
</body>
</html>
`
