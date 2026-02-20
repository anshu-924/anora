package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

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

	// Look up connection by unique ID (format: client_id_device_id)
	conn, err := s.connHandler.GetDeviceByUniqueID(connectionID)
	if err != nil {
		return fmt.Errorf("connection not found: %s", connectionID)
	}

	// Attach stream to the connection
	if err := s.connHandler.AttachStream(conn.ClientID, conn.DeviceID, stream); err != nil {
		return err
	}

	// Initialize heartbeat timestamp
	conn.LastHeartbeatAt = time.Now()
	conn.HeartbeatFailCount = 0

	// Create heartbeat stop channel and store it in connection
	conn.HeartbeatStopChan = make(chan bool, 1)

	log.Printf("Client %s (Device: %s) started streaming notifications", conn.ClientID, conn.DeviceID)

	// Start heartbeat goroutine
	go s.sendHeartbeats(conn, stream, conn.HeartbeatStopChan)

	// Keep the stream alive
	<-stream.Context().Done()

	// Stop heartbeat goroutine
	if conn.HeartbeatStopChan != nil {
		select {
		case conn.HeartbeatStopChan <- true:
			log.Printf("Signaled heartbeat stop for %s", conn.UniqueID)
		default:
			// Channel might already be stopped
		}
	}

	// Detach stream when client disconnects
	s.connHandler.DetachStream(conn.ClientID, conn.DeviceID)

	log.Printf("Client %s (Device: %s) disconnected from stream (Uptime: %v)",
		conn.ClientID, conn.DeviceID, conn.GetUptime())

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

// sendHeartbeats sends periodic heartbeat messages to the client
func (s *NotificationServer) sendHeartbeats(conn *models.Connection, stream pb.NotificationService_StreamNotificationsServer, done chan bool) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check if connection is still active before sending
			if !conn.IsActive || conn.Stream == nil {
				log.Printf("Connection %s is no longer active, stopping heartbeat", conn.UniqueID)
				return
			}

			heartbeat := &pb.Notification{
				Id:           fmt.Sprintf("heartbeat_%d", time.Now().Unix()),
				ConnectionId: conn.UniqueID,
				Title:        "heartbeat",
				Message:      "ping",
				ServiceName:  "system",
				Timestamp:    time.Now().Unix(),
				Type:         "heartbeat",
			}

			if err := stream.Send(heartbeat); err != nil {
				conn.HeartbeatFailCount++
				log.Printf("Failed to send heartbeat to %s (fail count: %d): %v",
					conn.UniqueID, conn.HeartbeatFailCount, err)

				// If failed twice, disconnect the device
				if conn.HeartbeatFailCount >= 2 {
					log.Printf("Heartbeat failed twice for %s, disconnecting device", conn.UniqueID)
					s.connHandler.UnregisterDevice(conn.ClientID, conn.DeviceID)
					return
				}
			} else {
				// Reset fail count on successful heartbeat
				conn.HeartbeatFailCount = 0
				conn.LastHeartbeatAt = time.Now()
				log.Printf("Heartbeat sent to %s", conn.UniqueID)
			}

		case <-done:
			log.Printf("Stopping heartbeat for %s", conn.UniqueID)
			return
		}
	}
}
