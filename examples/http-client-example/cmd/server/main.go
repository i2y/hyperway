package main

import (
	"log"
	"net/http"
	"time"

	"github.com/i2y/hyperway/examples/http-client-example/server"
	"github.com/i2y/hyperway/rpc"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func main() {
	userService := server.NewUserService()

	svc := rpc.NewService("user.v1",
		rpc.WithValidation(true),
	)

	rpc.MustRegister(svc, "CreateUser", userService.CreateUser)
	rpc.MustRegister(svc, "GetUser", userService.GetUser)
	rpc.MustRegister(svc, "ListUsers", userService.ListUsers)
	rpc.MustRegister(svc, "UpdateUser", userService.UpdateUser)
	rpc.MustRegister(svc, "DeleteUser", userService.DeleteUser)

	gateway, err := rpc.NewGateway(svc)
	if err != nil {
		log.Fatalf("Failed to create gateway: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", gateway)

	h2s := &http2.Server{}
	handler := h2c.NewHandler(mux, h2s)

	httpServer := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Println("Starting Hyperway server on :8080")
	log.Println("Service: user.v1")
	log.Println("Protocols: gRPC, Connect, gRPC-Web")

	if err := httpServer.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
