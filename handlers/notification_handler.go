package handlers

import (
	"context"
	"fmt"
	"log"

	"grpcon/models"
	pb "grpcon/proto"
)

// NotificationServer implements the NotificationService gRPC server
type NotificationServer struct {
	pb.UnimplementedNotificationServiceServer
	connHandler *ConnectionHandler
}

// NewNotificationServer creates a new notification server instance
func NewNotificationServer() *NotificationServer {
	return &NotificationServer{
		connHandler: NewConnectionHandler(),
	}
}

// AddConnection handles adding a new device connection
func (s *NotificationServer) AddConnection(ctx context.Context, req *pb.ConnectionRequest) (*pb.ConnectionResponse, error) {
	// Extract client_id and device_id from connection_id (format: client_id_device_id)
	// Or pass them separately - for now we'll parse the connection_id

	if req.ConnectionId == "" {
		return &pb.ConnectionResponse{
			Success: false,
			Message: "connection_id is required (format: client_id_device_id)",
		}, nil
	}

	// For simplicity, we'll use connection_id as client_id and service_name as device_id
	// In a real implementation, you'd parse these properly
	// Format expected: connection_id contains both client and device info

	clientID := req.ConnectionId
	deviceID := req.ServiceName // Using service_name field as device_id for now

	if deviceID == "" {
		deviceID = "default_device"
	}

	conn, err := s.connHandler.RegisterDevice(clientID, deviceID, req.ServiceName)
	if err != nil {
		return &pb.ConnectionResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.ConnectionResponse{
		Success:      true,
		Message:      "device registered successfully",
		ConnectionId: conn.UniqueID,
	}, nil
}

// RemoveConnection handles removing a device connection
func (s *NotificationServer) RemoveConnection(ctx context.Context, req *pb.ConnectionRequest) (*pb.ConnectionResponse, error) {
	if req.ConnectionId == "" {
		return &pb.ConnectionResponse{
			Success: false,
			Message: "connection_id is required",
		}, nil
	}

	clientID := req.ConnectionId
	deviceID := req.ServiceName
	if deviceID == "" {
		deviceID = "default_device"
	}

	err := s.connHandler.UnregisterDevice(clientID, deviceID)
	if err != nil {
		return &pb.ConnectionResponse{
			Success:      false,
			Message:      err.Error(),
			ConnectionId: req.ConnectionId,
		}, nil
	}

	return &pb.ConnectionResponse{
		Success:      true,
		Message:      "device unregistered successfully",
		ConnectionId: req.ConnectionId,
	}, nil
}

// StreamNotifications handles streaming notifications to connected devices
func (s *NotificationServer) StreamNotifications(req *pb.SubscribeRequest, stream pb.NotificationService_StreamNotificationsServer) error {
	connectionID := req.ConnectionId

	if connectionID == "" {
		return fmt.Errorf("connection_id is required")
	}

	// Parse connection_id to get client_id and device_id
	// For now using simple split logic - improve based on your ID format
	clientID := connectionID
	deviceID := "default_device"

	// Check if connection exists
	conn, err := s.connHandler.GetDeviceInfo(clientID, deviceID)
	if err != nil {
		return fmt.Errorf("connection not found: %s", connectionID)
	}

	// Attach stream to the connection
	if err := s.connHandler.AttachStream(clientID, deviceID, stream); err != nil {
		return err
	}

	log.Printf("Client %s (Device: %s) started streaming notifications", clientID, deviceID)

	// Keep the stream alive
	<-stream.Context().Done()

	// Detach stream when client disconnects
	s.connHandler.DetachStream(clientID, deviceID)

	log.Printf("Client %s (Device: %s) disconnected from stream (Uptime: %v)",
		clientID, deviceID, conn.GetUptime())

	return nil
}

// SendNotificationToClient sends notification to all devices of a specific client
func (s *NotificationServer) SendNotificationToClient(notification *models.NotificationData) error {
	return s.connHandler.SendNotificationToClient(notification)
}

// BroadcastNotification sends a notification to all connected clients and devices
func (s *NotificationServer) BroadcastNotification(notification *models.NotificationData) {
	s.connHandler.BroadcastToAll(notification)
}

// GetConnectionHandler returns the connection handler instance
func (s *NotificationServer) GetConnectionHandler() *ConnectionHandler {
	return s.connHandler
}

// GetConnectionStats returns connection statistics
func (s *NotificationServer) GetConnectionStats() map[string]interface{} {
	return s.connHandler.GetConnectionStats()
}
