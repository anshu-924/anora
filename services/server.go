package services

import (
	"log"
	"net"

	"grpcon/handlers"
	pb "grpcon/proto"

	"google.golang.org/grpc"
)

// Server wraps the gRPC server and notification handler
type Server struct {
	grpcServer         *grpc.Server
	notificationServer *handlers.NotificationServer
	listener           net.Listener
}

// NewServer creates a new gRPC server instance
func NewServer(port string) (*Server, error) {
	// Create listener
	lis, err := net.Listen("tcp", port)
	if err != nil {
		return nil, err
	}

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Create notification server handler
	notificationServer := handlers.NewNotificationServer()

	// Register the service
	pb.RegisterNotificationServiceServer(grpcServer, notificationServer)

	log.Printf("gRPC server initialized on %s", port)

	return &Server{
		grpcServer:         grpcServer,
		notificationServer: notificationServer,
		listener:           lis,
	}, nil
}

// Start begins serving gRPC requests
func (s *Server) Start() error {
	log.Printf("Starting gRPC server on %s", s.listener.Addr().String())
	return s.grpcServer.Serve(s.listener)
}

// Stop gracefully stops the gRPC server
func (s *Server) Stop() {
	log.Println("Stopping gRPC server...")
	s.grpcServer.GracefulStop()
}

// GetNotificationServer returns the notification server handler
func (s *Server) GetNotificationServer() *handlers.NotificationServer {
	return s.notificationServer
}
