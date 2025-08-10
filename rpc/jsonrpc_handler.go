package rpc

import (
	"net/http"
)

// JSONRPCHandler returns an HTTP handler for JSON-RPC requests.
// This handler processes all JSON-RPC requests at a single endpoint.
func (s *Service) JSONRPCHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a dummy handler context since we don't know the method yet
		ctx := &handlerContext{
			options:          s.options,
			validator:        s.validator,
			responseHeaders:  make(map[string][]string),
			responseTrailers: make(map[string][]string),
			requestHeaders:   r.Header,
		}

		// Handle the JSON-RPC request
		s.handleJSONRPCRequest(w, r, ctx)
	})
}
