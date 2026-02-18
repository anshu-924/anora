package models

import (
	"fmt"
	"sync"
	"time"

	pb "grpcon/proto"
)

// Connection represents an active device connection
type Connection struct {
	UniqueID           string // client_id_device_id
	ClientID           string
	DeviceID           string
	ServiceName        string
	Stream             pb.NotificationService_StreamNotificationsServer
	ConnectedAt        time.Time
	LastNotificationAt time.Time
	NotificationCount  int
	IsActive           bool
}

// GetUptime returns how long the connection has been active
func (c *Connection) GetUptime() time.Duration {
	return time.Since(c.ConnectedAt)
}

// ClientGroup represents all devices connected for a single client
type ClientGroup struct {
	ClientID string
	Devices  map[string]*Connection // key: device_id
	mu       sync.RWMutex
}

// NewClientGroup creates a new client group
func NewClientGroup(clientID string) *ClientGroup {
	return &ClientGroup{
		ClientID: clientID,
		Devices:  make(map[string]*Connection),
	}
}

// AddDevice adds a device connection to this client group
func (cg *ClientGroup) AddDevice(conn *Connection) {
	cg.mu.Lock()
	defer cg.mu.Unlock()
	cg.Devices[conn.DeviceID] = conn
}

// RemoveDevice removes a device from this client group
func (cg *ClientGroup) RemoveDevice(deviceID string) bool {
	cg.mu.Lock()
	defer cg.mu.Unlock()
	if _, exists := cg.Devices[deviceID]; exists {
		delete(cg.Devices, deviceID)
		return true
	}
	return false
}

// GetDevice retrieves a specific device connection
func (cg *ClientGroup) GetDevice(deviceID string) (*Connection, bool) {
	cg.mu.RLock()
	defer cg.mu.RUnlock()
	conn, exists := cg.Devices[deviceID]
	return conn, exists
}

// GetAllDevices returns all device connections for this client
func (cg *ClientGroup) GetAllDevices() []*Connection {
	cg.mu.RLock()
	defer cg.mu.RUnlock()
	devices := make([]*Connection, 0, len(cg.Devices))
	for _, device := range cg.Devices {
		devices = append(devices, device)
	}
	return devices
}

// GetDeviceCount returns number of devices connected for this client
func (cg *ClientGroup) GetDeviceCount() int {
	cg.mu.RLock()
	defer cg.mu.RUnlock()
	return len(cg.Devices)
}

// ConnectionManager manages all client groups and their device connections
type ConnectionManager struct {
	mu      sync.RWMutex
	clients map[string]*ClientGroup // key: client_id
}

// NewConnectionManager creates a new connection manager instance
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		clients: make(map[string]*ClientGroup),
	}
}

// AddConnection adds a new device connection, grouped by client_id
func (cm *ConnectionManager) AddConnection(conn *Connection) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Get or create client group
	clientGroup, exists := cm.clients[conn.ClientID]
	if !exists {
		clientGroup = NewClientGroup(conn.ClientID)
		cm.clients[conn.ClientID] = clientGroup
	}

	// Add device to client group
	clientGroup.AddDevice(conn)
}

// RemoveConnection removes a device connection using unique_id (client_id_device_id)
func (cm *ConnectionManager) RemoveConnection(uniqueID string, clientID string, deviceID string) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	clientGroup, exists := cm.clients[clientID]
	if !exists {
		return false
	}

	removed := clientGroup.RemoveDevice(deviceID)

	// If client has no more devices, remove the client group
	if clientGroup.GetDeviceCount() == 0 {
		delete(cm.clients, clientID)
	}

	return removed
}

// GetConnection retrieves a specific device connection
func (cm *ConnectionManager) GetConnection(clientID string, deviceID string) (*Connection, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	clientGroup, exists := cm.clients[clientID]
	if !exists {
		return nil, false
	}

	return clientGroup.GetDevice(deviceID)
}

// GetClientGroup retrieves all devices for a specific client
func (cm *ConnectionManager) GetClientGroup(clientID string) (*ClientGroup, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	clientGroup, exists := cm.clients[clientID]
	return clientGroup, exists
}

// GetAllClientIDs returns list of all client IDs
func (cm *ConnectionManager) GetAllClientIDs() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	clientIDs := make([]string, 0, len(cm.clients))
	for clientID := range cm.clients {
		clientIDs = append(clientIDs, clientID)
	}
	return clientIDs
}

// GetAllConnections returns all device connections across all clients
func (cm *ConnectionManager) GetAllConnections() []*Connection {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var connections []*Connection
	for _, clientGroup := range cm.clients {
		connections = append(connections, clientGroup.GetAllDevices()...)
	}
	return connections
}

// GetTotalDeviceCount returns total number of connected devices
func (cm *ConnectionManager) GetTotalDeviceCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	count := 0
	for _, clientGroup := range cm.clients {
		count += clientGroup.GetDeviceCount()
	}
	return count
}

// GetClientCount returns number of unique clients
func (cm *ConnectionManager) GetClientCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.clients)
}

// GetStats returns connection statistics
func (cm *ConnectionManager) GetStats() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["total_clients"] = len(cm.clients)

	totalDevices := 0
	for _, clientGroup := range cm.clients {
		totalDevices += clientGroup.GetDeviceCount()
	}
	stats["total_devices"] = totalDevices

	return stats
}

// NotificationData represents notification information
type NotificationData struct {
	ID          string
	ClientID    string // Target client (all their devices will receive)
	Title       string
	Message     string
	ServiceName string
	Timestamp   int64
}

// ToProto converts NotificationData to protobuf Notification
func (n *NotificationData) ToProto(connectionID string) *pb.Notification {
	return &pb.Notification{
		Id:           n.ID,
		ConnectionId: connectionID,
		Title:        n.Title,
		Message:      n.Message,
		ServiceName:  n.ServiceName,
		Timestamp:    n.Timestamp,
	}
}

// CreateUniqueID generates unique connection ID from client_id and device_id
func CreateUniqueID(clientID, deviceID string) string {
	return fmt.Sprintf("%s_%s", clientID, deviceID)
}
