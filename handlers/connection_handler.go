package handlers

import (
	"fmt"
	"log"
	"time"

	"grpcon/models"
	pb "grpcon/proto"
)

// ConnectionHandler manages device connections grouped by client
type ConnectionHandler struct {
	connManager *models.ConnectionManager
}

// NewConnectionHandler creates a new connection handler
func NewConnectionHandler() *ConnectionHandler {
	return &ConnectionHandler{
		connManager: models.NewConnectionManager(),
	}
}

// RegisterDevice registers a new device connection for a client
func (h *ConnectionHandler) RegisterDevice(clientID, deviceID, serviceName string) (*models.Connection, error) {
	if clientID == "" {
		return nil, fmt.Errorf("client_id is required")
	}
	if deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	uniqueID := models.CreateUniqueID(clientID, deviceID)

	// Check if this device is already connected
	if existingConn, exists := h.connManager.GetConnection(clientID, deviceID); exists {
		log.Printf("Device already connected: %s", uniqueID)
		return existingConn, nil
	}

	// Create new connection
	conn := &models.Connection{
		UniqueID:          uniqueID,
		ClientID:          clientID,
		DeviceID:          deviceID,
		ServiceName:       serviceName,
		ConnectedAt:       time.Now(),
		NotificationCount: 0,
		IsActive:          false, //will be set to true when stream is attached
	}

	h.connManager.AddConnection(conn)

	log.Printf("Device registered: %s (Client: %s, Device: %s, Service: %s)",
		uniqueID, clientID, deviceID, serviceName)

	return conn, nil
}

// UnregisterDevice removes a device connection
func (h *ConnectionHandler) UnregisterDevice(clientID, deviceID string) error {
	if clientID == "" || deviceID == "" {
		return fmt.Errorf("client_id and device_id are required")
	}

	uniqueID := models.CreateUniqueID(clientID, deviceID)
	 // Get connection before removing to check stream status
    conn, exists := h.connManager.GetConnection(clientID, deviceID)
    if !exists {
        return fmt.Errorf("device not found: %s", uniqueID)
    }
    
    // If stream is attached and active, close it gracefully
    if conn.Stream != nil && conn.IsActive {
        log.Printf("Closing active stream for device: %s", uniqueID)
        // The stream will be closed when we return from StreamNotifications
        // Mark as inactive first
        conn.IsActive = false
        conn.Stream = nil
    }
	removed := h.connManager.RemoveConnection(uniqueID, clientID, deviceID)

	if !removed {
		return fmt.Errorf("device not found: %s", uniqueID)
	}

	log.Printf("Device unregistered: %s", uniqueID)
	return nil
}

// AttachStream attaches a gRPC stream to an existing device connection
func (h *ConnectionHandler) AttachStream(clientID, deviceID string, stream pb.NotificationService_StreamNotificationsServer) error {
	conn, exists := h.connManager.GetConnection(clientID, deviceID)
	if !exists {
		return fmt.Errorf("connection not found for client: %s, device: %s", clientID, deviceID)
	}

	conn.Stream = stream
	conn.IsActive = true

	log.Printf("Stream attached to device: %s", conn.UniqueID)
	return nil
}

// DetachStream marks a device's stream as inactive
func (h *ConnectionHandler) DetachStream(clientID, deviceID string) {
	conn, exists := h.connManager.GetConnection(clientID, deviceID)
	if exists {
		conn.Stream = nil
		conn.IsActive = false
		log.Printf("Stream detached from device: %s", conn.UniqueID)
	}
}

// GetClientDevices returns all devices for a specific client
func (h *ConnectionHandler) GetClientDevices(clientID string) ([]*models.Connection, error) {
	clientGroup, exists := h.connManager.GetClientGroup(clientID)
	if !exists {
		return nil, fmt.Errorf("no devices found for clientt: %s", clientID)
	}

	return clientGroup.GetAllDevices(), nil
}

// GetDeviceInfo retrieves information about a specific device
func (h *ConnectionHandler) GetDeviceInfo(clientID, deviceID string) (*models.Connection, error) {
	conn, exists := h.connManager.GetConnection(clientID, deviceID)
	if !exists {
		return nil, fmt.Errorf("device not found")
	}
	return conn, nil
}

// GetDeviceByUniqueID retrieves a device by its unique ID
func (h *ConnectionHandler) GetDeviceByUniqueID(uniqueID string) (*models.Connection, error) {
	conn, exists := h.connManager.GetConnectionByUniqueID(uniqueID)
	if !exists {
		return nil, fmt.Errorf("device not found: %s", uniqueID)
	}
	return conn, nil
}

// GetConnectionStats returns statistics about connections
func (h *ConnectionHandler) GetConnectionStats() map[string]interface{} {
	stats := h.connManager.GetStats()
	stats["client_ids"] = h.connManager.GetAllClientIDs()
	return stats
}

// SendNotificationToClient sends notification to all devices of a client
func (h *ConnectionHandler) SendNotificationToClient(notification *models.NotificationData) error {
	clientGroup, exists := h.connManager.GetClientGroup(notification.ClientID)
	if !exists {
		return fmt.Errorf("no devices found for client: %s", notification.ClientID)
	}

	devices := clientGroup.GetAllDevices()
	// log.Printf("Sending notification to client %s with %d devices", notification.ClientID, len(devices))
	// log.Printf(" devices: %v", devices)
	successCount := 0
	failCount := 0

	for _, device := range devices {
		// log.Printf(" single device data: %v", device)
		if device.Stream != nil && device.IsActive {
			// Send notification
			if err := device.Stream.Send(notification.ToProto(device.UniqueID)); err != nil {
				log.Printf("Failed to send notification to %s: %v", device.UniqueID, err)
				failCount++
			} else {
				// Update last notification time and count
				device.LastNotificationAt = time.Now()
				device.NotificationCount++
				successCount++
			}
		} else {
			log.Printf("Device %s has no active stream", device.UniqueID)
			failCount++
		}
	}

	log.Printf("Notification sent to client %s: %d success, %d failed (total devices: %d)",
		notification.ClientID, successCount, failCount, len(devices))

	if successCount == 0 {
		return fmt.Errorf("failed to send notification to any device")
	}

	return nil
}

// BroadcastToAll sends notification to all connected clients and their devices
func (h *ConnectionHandler) BroadcastToAll(notification *models.NotificationData) {
	clientIDs := h.connManager.GetAllClientIDs()
	totalDevices := 0
	successCount := 0

	for _, clientID := range clientIDs {
		clientGroup, exists := h.connManager.GetClientGroup(clientID)
		if !exists {
			continue
		}

		devices := clientGroup.GetAllDevices()
		totalDevices += len(devices)

		for _, device := range devices {
			if device.Stream != nil && device.IsActive {
				notif := &models.NotificationData{
					ID:          notification.ID,
					ClientID:    device.ClientID,
					Title:       notification.Title,
					Message:     notification.Message,
					ServiceName: notification.ServiceName,
					Timestamp:   notification.Timestamp,
				}

				if err := device.Stream.Send(notif.ToProto(device.UniqueID)); err != nil {
					log.Printf("Failed to broadcast to %s: %v", device.UniqueID, err)
				} else {
					device.LastNotificationAt = time.Now()
					device.NotificationCount++
					successCount++
				}
			}
		}
	}

	log.Printf("Broadcast complete: %d/%d devices notified across %d clients",
		successCount, totalDevices, len(clientIDs))
}

// GetConnectionManager returns the underlying connection manager
func (h *ConnectionHandler) GetConnectionManager() *models.ConnectionManager {
	return h.connManager
}
