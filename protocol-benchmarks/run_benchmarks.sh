#!/bin/bash

# Comprehensive benchmark script for Hyperway vs Connect-go
# This script runs both Go benchmarks and Apache Bench tests

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
HYPERWAY_PORT=8080
CONNECT_GO_PORT=8084
BENCHMARK_TIME="10s"
APACHE_BENCH_REQUESTS=10000
APACHE_BENCH_CONCURRENCY=100
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RESULTS_DIR="benchmark_results"

# Function to print colored messages
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_header() {
    echo -e "${BLUE}=== $1 ===${NC}"
}

# Function to check if a port is in use
check_port() {
    local port=$1
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Function to kill processes on specific ports
cleanup_servers() {
    print_status "Cleaning up any existing servers..."
    pkill -f hyperway_bench_server 2>/dev/null || true
    pkill -f connect_bench_server 2>/dev/null || true
    sleep 2
}

# Function to build servers
build_servers() {
    print_header "Building benchmark servers"
    
    print_status "Building Hyperway benchmark server..."
    go build -o hyperway_bench_server hyperway_server.go
    
    print_status "Building Connect-go benchmark server..."
    go build -o connect_bench_server connect_server.go
    
    print_status "Build complete"
}

# Function to start servers
start_servers() {
    print_header "Starting benchmark servers"
    
    print_status "Starting Hyperway server on port $HYPERWAY_PORT..."
    ./hyperway_bench_server > hyperway_server.log 2>&1 &
    HYPERWAY_PID=$!
    
    print_status "Starting Connect-go server on port $CONNECT_GO_PORT..."
    ./connect_bench_server > connect_server.log 2>&1 &
    CONNECT_PID=$!
    
    # Wait for servers to start
    print_status "Waiting for servers to start..."
    sleep 3
    
    # Verify servers are running
    if ! check_port $HYPERWAY_PORT; then
        print_error "Hyperway server failed to start on port $HYPERWAY_PORT"
        cat hyperway_server.log
        exit 1
    fi
    
    if ! check_port $CONNECT_GO_PORT; then
        print_error "Connect-go server failed to start on port $CONNECT_GO_PORT"
        cat connect_server.log
        exit 1
    fi
    
    print_status "Both servers are running successfully"
}

# Function to run Go benchmarks
run_go_benchmarks() {
    print_header "Running Go benchmarks"
    
    local output_file="$RESULTS_DIR/go_benchmarks_$TIMESTAMP.txt"
    
    print_status "Running benchmarks for $BENCHMARK_TIME..."
    print_status "Output will be saved to: $output_file"
    
    go test -bench=. -benchmem -benchtime=$BENCHMARK_TIME 2>&1 | tee "$output_file"
    
    # Generate summary
    print_header "Go Benchmark Summary"
    echo ""
    echo "Unary RPC Performance:"
    grep -E "^Benchmark.*GRPC-14|^Benchmark.*Connect-14" "$output_file" | grep -v "Streaming" | grep -v "HTTP2" || true
    echo ""
    echo "Streaming RPC Performance:"
    grep -E "^Benchmark.*Streaming-14" "$output_file" || true
    echo ""
    echo "HTTP/2 Performance:"
    grep -E "^Benchmark.*HTTP2-14" "$output_file" || true
}

# Function to run Apache Bench tests
run_apache_bench() {
    print_header "Running Apache Bench tests"
    
    local output_file="$RESULTS_DIR/apache_bench_$TIMESTAMP.txt"
    local endpoint="/grpcweb.example.v1.GreeterService/Greet"
    local payload='{"name":"Benchmark Test"}'
    
    print_status "Testing Connect+JSON protocol with Apache Bench..."
    print_status "Requests: $APACHE_BENCH_REQUESTS, Concurrency: $APACHE_BENCH_CONCURRENCY"
    print_status "Output will be saved to: $output_file"
    
    {
        echo "=== Apache Bench Results ===" 
        echo "Timestamp: $(date)"
        echo ""
        
        echo "--- Connect-go Server (Port $CONNECT_GO_PORT) ---"
        ab -n $APACHE_BENCH_REQUESTS -c $APACHE_BENCH_CONCURRENCY -k \
           -p /dev/stdin \
           -T "application/json" \
           http://127.0.0.1:${CONNECT_GO_PORT}${endpoint} <<< "$payload" 2>&1
        
        echo ""
        echo "--- Hyperway Server (Port $HYPERWAY_PORT) ---"
        ab -n $APACHE_BENCH_REQUESTS -c $APACHE_BENCH_CONCURRENCY -k \
           -p /dev/stdin \
           -T "application/json" \
           http://127.0.0.1:${HYPERWAY_PORT}${endpoint} <<< "$payload" 2>&1
    } | tee "$output_file"
    
    # Extract and display key metrics
    print_header "Apache Bench Summary"
    echo ""
    echo "Connect-go Performance:"
    grep "Requests per second" "$output_file" | head -1 || true
    grep "Time per request.*mean" "$output_file" | head -1 || true
    echo ""
    echo "Hyperway Performance:"
    grep "Requests per second" "$output_file" | tail -1 || true
    grep "Time per request.*mean" "$output_file" | tail -1 || true
}

# Function to generate comparison report
generate_report() {
    print_header "Generating comparison report"
    
    local report_file="$RESULTS_DIR/benchmark_report_$TIMESTAMP.md"
    
    cat > "$report_file" << EOF
# Benchmark Report - $(date)

## Environment
- Platform: $(uname -s)
- Architecture: $(uname -m)
- CPU: $(sysctl -n machdep.cpu.brand_string 2>/dev/null || echo "Unknown")
- Go Version: $(go version)

## Test Configuration
- Go Benchmark Duration: $BENCHMARK_TIME
- Apache Bench Requests: $APACHE_BENCH_REQUESTS
- Apache Bench Concurrency: $APACHE_BENCH_CONCURRENCY

## Results Summary

### Go Benchmarks
See detailed results in: go_benchmarks_$TIMESTAMP.txt

### Apache Bench Tests  
See detailed results in: apache_bench_$TIMESTAMP.txt

## Performance Comparison

EOF

    # Parse and add key metrics to report
    if [ -f "$RESULTS_DIR/go_benchmarks_$TIMESTAMP.txt" ]; then
        echo "### Unary RPC (ns/op)" >> "$report_file"
        echo '```' >> "$report_file"
        grep -E "^Benchmark(ConnectGo|Hyperway)GRPC-14" "$RESULTS_DIR/go_benchmarks_$TIMESTAMP.txt" | head -2 >> "$report_file"
        grep -E "^Benchmark(ConnectGo|Hyperway)Connect-14" "$RESULTS_DIR/go_benchmarks_$TIMESTAMP.txt" | grep -v HTTP2 | head -2 >> "$report_file"
        echo '```' >> "$report_file"
        echo "" >> "$report_file"
        
        echo "### Streaming RPC (ns/op)" >> "$report_file"
        echo '```' >> "$report_file"
        grep -E "Streaming-14" "$RESULTS_DIR/go_benchmarks_$TIMESTAMP.txt" >> "$report_file"
        echo '```' >> "$report_file"
        echo "" >> "$report_file"
    fi
    
    print_status "Report generated: $report_file"
}

# Function to display usage
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -g, --go-only          Run only Go benchmarks"
    echo "  -a, --apache-only      Run only Apache Bench tests"
    echo "  -t, --time DURATION    Set benchmark duration (default: 10s)"
    echo "  -r, --requests NUM     Set Apache Bench requests (default: 10000)"
    echo "  -c, --concurrency NUM  Set Apache Bench concurrency (default: 100)"
    echo "  -h, --help            Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                    # Run all benchmarks"
    echo "  $0 --go-only          # Run only Go benchmarks"
    echo "  $0 --time 30s         # Run benchmarks for 30 seconds"
    echo "  $0 -r 50000 -c 200    # Apache Bench with 50k requests, 200 concurrent"
}

# Parse command line arguments
RUN_GO=true
RUN_APACHE=true

while [[ $# -gt 0 ]]; do
    case $1 in
        -g|--go-only)
            RUN_APACHE=false
            shift
            ;;
        -a|--apache-only)
            RUN_GO=false
            shift
            ;;
        -t|--time)
            BENCHMARK_TIME="$2"
            shift 2
            ;;
        -r|--requests)
            APACHE_BENCH_REQUESTS="$2"
            shift 2
            ;;
        -c|--concurrency)
            APACHE_BENCH_CONCURRENCY="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Main execution
main() {
    print_header "Hyperway vs Connect-go Benchmark Suite"
    echo ""
    
    # Create results directory
    mkdir -p "$RESULTS_DIR"
    
    # Clean up any existing servers
    cleanup_servers
    
    # Build servers
    build_servers
    
    # Start servers
    start_servers
    
    # Run benchmarks
    if [ "$RUN_GO" = true ]; then
        run_go_benchmarks
    fi
    
    if [ "$RUN_APACHE" = true ]; then
        # Check if ab is installed
        if ! command -v ab &> /dev/null; then
            print_warning "Apache Bench (ab) is not installed. Skipping Apache Bench tests."
            print_warning "Install with: brew install apache2-utils (macOS) or apt-get install apache2-utils (Linux)"
        else
            run_apache_bench
        fi
    fi
    
    # Generate report
    generate_report
    
    # Cleanup
    print_header "Cleaning up"
    print_status "Stopping servers..."
    kill $HYPERWAY_PID 2>/dev/null || true
    kill $CONNECT_PID 2>/dev/null || true
    
    print_header "Benchmark Complete"
    print_status "Results saved in: $RESULTS_DIR/"
    print_status "Report: $RESULTS_DIR/benchmark_report_$TIMESTAMP.md"
    
    # Show quick comparison
    echo ""
    print_header "Quick Performance Comparison"
    echo ""
    
    if [ -f "$RESULTS_DIR/go_benchmarks_$TIMESTAMP.txt" ]; then
        echo "Streaming Performance (lower is better):"
        echo -n "  Connect-go: "
        grep "BenchmarkConnectGoStreaming-14" "$RESULTS_DIR/go_benchmarks_$TIMESTAMP.txt" | awk '{print $3 " ns/op"}' || echo "N/A"
        echo -n "  Hyperway:   "
        grep "BenchmarkHyperwayStreaming-14" "$RESULTS_DIR/go_benchmarks_$TIMESTAMP.txt" | awk '{print $3 " ns/op"}' || echo "N/A"
        
        # Calculate improvement
        connect_streaming=$(grep "BenchmarkConnectGoStreaming-14" "$RESULTS_DIR/go_benchmarks_$TIMESTAMP.txt" | awk '{print $3}' || echo "0")
        hyperway_streaming=$(grep "BenchmarkHyperwayStreaming-14" "$RESULTS_DIR/go_benchmarks_$TIMESTAMP.txt" | awk '{print $3}' || echo "0")
        
        if [ "$connect_streaming" != "0" ] && [ "$hyperway_streaming" != "0" ]; then
            improvement=$(echo "scale=1; (($connect_streaming - $hyperway_streaming) * 100) / $connect_streaming" | bc)
            echo -e "  ${GREEN}Improvement: ${improvement}% faster${NC}"
        fi
    fi
}

# Trap to ensure cleanup on exit
trap 'cleanup_servers' EXIT

# Run main function
main
