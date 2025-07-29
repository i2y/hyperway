package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/i2y/hyperway/rpc"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// CountRequest represents a request to count
type CountRequest struct {
	UpTo int `json:"up_to"`
}

// CountResponse represents a count response
type CountResponse struct {
	Number    int       `json:"number"`
	Timestamp time.Time `json:"timestamp"`
}

// TimeRequest represents a request for current time
type TimeRequest struct {
	Interval int `json:"interval_seconds"`
	Count    int `json:"count"`
}

// TimeResponse represents a time response
type TimeResponse struct {
	CurrentTime time.Time `json:"current_time"`
	Message     string    `json:"message"`
}

// Service handlers
func handleCount(ctx context.Context, req *CountRequest, stream rpc.ServerStream[CountResponse]) error {
	log.Printf("Count request: up to %d", req.UpTo)

	// Get handler context to set headers
	if hctx := rpc.GetHandlerContext(ctx); hctx != nil {
		hctx.SetResponseHeader("X-Stream-Type", "count")
	}

	for i := 1; i <= req.UpTo; i++ {
		resp := &CountResponse{
			Number:    i,
			Timestamp: time.Now(),
		}

		if err := stream.Send(resp); err != nil {
			return err
		}

		// Small delay to simulate work
		time.Sleep(100 * time.Millisecond)
	}

	// Set trailer
	if hctx := rpc.GetHandlerContext(ctx); hctx != nil {
		hctx.SetResponseTrailer("X-Total-Count", fmt.Sprintf("%d", req.UpTo))
	}

	return nil
}

func handleTime(ctx context.Context, req *TimeRequest, stream rpc.ServerStream[TimeResponse]) error {
	log.Printf("Time request: %d updates every %d seconds", req.Count, req.Interval)

	// Get handler context to set headers
	if hctx := rpc.GetHandlerContext(ctx); hctx != nil {
		hctx.SetResponseHeader("X-Stream-Type", "time")
	}

	ticker := time.NewTicker(time.Duration(req.Interval) * time.Second)
	defer ticker.Stop()

	for i := 0; i < req.Count; i++ {
		resp := &TimeResponse{
			CurrentTime: time.Now(),
			Message:     fmt.Sprintf("Update %d of %d", i+1, req.Count),
		}

		if err := stream.Send(resp); err != nil {
			return err
		}

		if i < req.Count-1 {
			select {
			case <-ticker.C:
				// Continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return nil
}

func main() {
	// Create service
	svc := rpc.NewService("StreamingExample",
		rpc.WithPackage("examples.streaming.v1"),
		rpc.WithReflection(true))

	// Register server-streaming methods using type-safe registration
	rpc.MustRegisterServerStream(svc, "Count", handleCount)
	rpc.MustRegisterServerStream(svc, "Time", handleTime)

	// Create gateway
	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		log.Fatal(err)
	}

	// Create HTTP server
	mux := http.NewServeMux()
	mux.Handle("/", gateway)

	// Add a simple HTML page for testing
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, testHTML)
	})

	log.Println("Streaming server starting on :8080")
	log.Println("Test page: http://localhost:8080/test")
	log.Println("")
	log.Println("Example requests:")
	log.Println("  Count (Connect): curl -X POST http://localhost:8080/examples.streaming.v1.StreamingExample/Count -H 'Content-Type: application/json' -d '{\"up_to\": 5}'")
	log.Println("  Time (Connect):  curl -X POST http://localhost:8080/examples.streaming.v1.StreamingExample/Time -H 'Content-Type: application/json' -d '{\"interval_seconds\": 1, \"count\": 3}'")
	log.Println("  Count (gRPC):    grpcurl -plaintext -d '{\"up_to\": 5}' localhost:8080 examples.streaming.v1.StreamingExample/Count")

	// Use h2c (HTTP/2 without TLS) for gRPC support
	h2s := &http2.Server{}
	handler := h2c.NewHandler(mux, h2s)

	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

const testHTML = `
<!DOCTYPE html>
<html>
<head>
    <title>Streaming Example</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .section { margin: 20px 0; padding: 20px; border: 1px solid #ccc; }
        button { margin: 10px 0; padding: 10px 20px; }
        pre { background: #f4f4f4; padding: 10px; overflow: auto; }
        .response { margin-top: 10px; }
    </style>
</head>
<body>
    <h1>Hyperway Streaming Example</h1>
    
    <div class="section">
        <h2>Count Stream</h2>
        <p>Streams numbers from 1 to N</p>
        <input type="number" id="countUpTo" value="5" min="1" max="20" />
        <button onclick="startCount()">Start Count</button>
        <div class="response">
            <pre id="countResponse"></pre>
        </div>
    </div>
    
    <div class="section">
        <h2>Time Stream</h2>
        <p>Streams current time at intervals</p>
        <label>Interval (seconds): <input type="number" id="timeInterval" value="1" min="1" max="10" /></label><br>
        <label>Count: <input type="number" id="timeCount" value="5" min="1" max="10" /></label><br>
        <button onclick="startTime()">Start Time Stream</button>
        <div class="response">
            <pre id="timeResponse"></pre>
        </div>
    </div>
    
    <script>
    async function startCount() {
        const upTo = document.getElementById('countUpTo').value;
        const responseEl = document.getElementById('countResponse');
        responseEl.textContent = 'Starting...\n';
        
        try {
            const response = await fetch('/examples.streaming.v1.StreamingExample/Count', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Connect-Protocol-Version': '1'
                },
                body: JSON.stringify({ up_to: parseInt(upTo) })
            });
            
            const reader = response.body.getReader();
            const decoder = new TextDecoder();
            let buffer = new Uint8Array();
            
            while (true) {
                const { done, value } = await reader.read();
                if (done) break;
                
                // Concatenate chunks
                const newBuffer = new Uint8Array(buffer.length + value.length);
                newBuffer.set(buffer);
                newBuffer.set(value, buffer.length);
                buffer = newBuffer;
                
                // Process complete messages
                while (buffer.length >= 5) {
                    const flags = buffer[0];
                    const length = (buffer[1] << 24) | (buffer[2] << 16) | (buffer[3] << 8) | buffer[4];
                    
                    if (buffer.length < 5 + length) break;
                    
                    const messageData = buffer.slice(5, 5 + length);
                    buffer = buffer.slice(5 + length);
                    
                    const message = JSON.parse(decoder.decode(messageData));
                    
                    if (flags === 0x02) {
                        // End of stream
                        responseEl.textContent += '\nStream ended\n';
                        break;
                    } else {
                        responseEl.textContent += JSON.stringify(message, null, 2) + '\n';
                    }
                }
            }
        } catch (error) {
            responseEl.textContent += 'Error: ' + error.message + '\n';
        }
    }
    
    async function startTime() {
        const interval = document.getElementById('timeInterval').value;
        const count = document.getElementById('timeCount').value;
        const responseEl = document.getElementById('timeResponse');
        responseEl.textContent = 'Starting...\n';
        
        try {
            const response = await fetch('/examples.streaming.v1.StreamingExample/Time', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Connect-Protocol-Version': '1'
                },
                body: JSON.stringify({ 
                    interval_seconds: parseInt(interval),
                    count: parseInt(count)
                })
            });
            
            const reader = response.body.getReader();
            const decoder = new TextDecoder();
            let buffer = new Uint8Array();
            
            while (true) {
                const { done, value } = await reader.read();
                if (done) break;
                
                // Concatenate chunks
                const newBuffer = new Uint8Array(buffer.length + value.length);
                newBuffer.set(buffer);
                newBuffer.set(value, buffer.length);
                buffer = newBuffer;
                
                // Process complete messages
                while (buffer.length >= 5) {
                    const flags = buffer[0];
                    const length = (buffer[1] << 24) | (buffer[2] << 16) | (buffer[3] << 8) | buffer[4];
                    
                    if (buffer.length < 5 + length) break;
                    
                    const messageData = buffer.slice(5, 5 + length);
                    buffer = buffer.slice(5 + length);
                    
                    const message = JSON.parse(decoder.decode(messageData));
                    
                    if (flags === 0x02) {
                        // End of stream
                        responseEl.textContent += '\nStream ended\n';
                        break;
                    } else {
                        responseEl.textContent += JSON.stringify(message, null, 2) + '\n';
                    }
                }
            }
        } catch (error) {
            responseEl.textContent += 'Error: ' + error.message + '\n';
        }
    }
    </script>
</body>
</html>
`
