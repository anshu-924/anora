# gRPC Connection Manager

A minimal gRPC server for managing client connections with device tracking and notification pushing capabilities. Supports multiple devices per client with automatic grouping.

## Quick Start (Easy Testing with HTTP Gateway)

The fastest way to test the notification system:

1. **Start the HTTP Gateway** (includes gRPC server):
   ```bash
   go run examples/http_gateway.go
   ```

2. **Open your browser** to `http://localhost:8080` for a simple UI

3. **Or use Postman/curl** to send notifications:
   ```bash
   # Send notification to a specific client
   curl -X POST http://localhost:8080/send \
     -H "Content-Type: application/json" \
     -d '{"client_id": "user123", "title": "Hello", "message": "Test notification"}'
   
   # Broadcast to all clients
   curl -X POST http://localhost:8080/broadcast \
     -H "Content-Type: application/json" \
     -d '{"title": "Announcement", "message": "System maintenance at 10 PM"}'
   
   # Get connection statistics
   curl http://localhost:8080/stats
   ```

4. **Test with a client** (in another terminal):
   ```bash
   go run examples/test_client.go
   ```

That's it! The HTTP gateway makes it easy to test notifications without writing gRPC code.

## Project Structure

```
grpcon/
├── proto/
│   ├── notification.proto          # Service definitions
│   ├── notification.pb.go          # Generated protobuf code
│   └── notification_grpc.pb.go     # Generated gRPC code
├── models/
│   └── models.go                   # Data models, client groups, and connection manager
├── handlers/
│   ├── connection_handler.go       # Connection management logic
│   └── notification_handler.go     # gRPC service implementation
├── services/
│   └── server.go                   # Server setup
├── examples/
│   ├── http_gateway.go             # HTTP gateway for easy testing
│   └── test_client.go              # Example gRPC client
├── main.go                         # Entry point
├── go.mod                          # Dependencies
└── README.md                       # Documentation
```

## Architecture

### Connection Structure

The server organizes connections in a hierarchical structure:

```
ConnectionManager
├── ClientGroup (client_id1)
│   ├── Device (device_id1) → UniqueID: client_id1_device_id1
│   ├── Device (device_id2) → UniqueID: client_id1_device_id2
│   └── Device (device_id3) → UniqueID: client_id1_device_id3
├── ClientGroup (client_id2)
│   ├── Device (device_id1) → UniqueID: client_id2_device_id1
│   └── Device (device_id2) → UniqueID: client_id2_device_id2
...
```

**Key Features:**
- Each unique connection has ID: `client_id_device_id`
- Multiple devices can connect for the same client
- Notifications sent to a `client_id` reach all their devices
- Track connection time, last notification time, notification count per device

### Tracked Metrics

Each device connection tracks:
- `ConnectedAt` - When the device connected
- `LastNotificationAt` - Last notification timestamp
- `NotificationCount` - Total notifications received
- `IsActive` - Whether stream is active
- `GetUptime()` - Connection duration

## Setup Instructions

### 1. Install Dependencies

```bash
go mod tidy
```

### 2. Install Protocol Buffer Compiler

Install `protoc` and the Go plugins:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### 3. Generate gRPC Code

```bash
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/notification.proto
```

### 4. Build the Server

```bash
go build -o grpcon.exe .
```

### 5. Run the Server

```bash
go run main.go
```

Or run the compiled executable:

```bash
./grpcon.exe
```

The server will start on port `50051` by default. You can change it by setting the `PORT` environment variable:

```bash
PORT=8080 go run main.go
```

## API Methods

### 1. AddConnection
Registers a new device connection for a client.

**Request:**
- `connection_id` - The client ID
- `service_name` - The device ID (or service identifier)

**Response:**
- `success` - Whether the operation succeeded
- `message` - Status message
- `connection_id` - The unique connection ID (client_id_device_id)

### 2. RemoveConnection
Unregisters a device connection.

**Request:**
- `connection_id` - The client ID
- `service_name` - The device ID

**Response:**
- `success` - Whether the operation succeeded
- `message` - Status message

### 3. StreamNotifications
Establishes a streaming connection to receive notifications from the server.

**Request:**
- `connection_id` - The unique connection ID or client ID

**Response:**
- Stream of `Notification` messages

## Testing with gRPCurl

### Install gRPCurl

```bash
# Windows (using scoop)
scoop install grpcurl

# Or download from: https://github.com/fullstorydev/grpcurl/releases
```

### Enable gRPC Reflection (Optional)

To use gRPCurl without specifying proto files, add reflection to [services/server.go](services/server.go):

```go
import "google.golang.org/grpc/reflection"

// In NewServer function, after RegisterNotificationServiceServer:
reflection.Register(grpcServer)
```

### Test Commands

#### 1. List Available Services

```bash
grpcurl -plaintext localhost:50051 list
```

#### 2. Register a Device Connection

```bash
grpcurl -plaintext -d "{\"connection_id\": \"user123\", \"service_name\": \"mobile_app\"}" localhost:50051 notification.NotificationService/AddConnection
```

#### 3. Remove a Device Connection

```bash
grpcurl -plaintext -d "{\"connection_id\": \"user123\", \"service_name\": \"mobile_app\"}" localhost:50051 notification.NotificationService/RemoveConnection
```

#### 4. Stream Notifications (Keep Alive)

```bash
grpcurl -plaintext -d "{\"connection_id\": \"user123\"}" localhost:50051 notification.NotificationService/StreamNotifications
```

This will keep the connection open and wait for notifications.

## Testing with Postman

Postman supports gRPC requests starting from version 8.5.0.

### Steps:

1. **Create a New gRPC Request**
   - Click "New" → "gRPC Request"

2. **Enter Server URL**
   ```
   localhost:50051
   ```

3. **Import Proto File**
   - Click "Import .proto" 
   - Select `proto/notification.proto`

4. **Test AddConnection**
   - Method: `notification.NotificationService/AddConnection`
   - Message:
   ```json
   {
     "connection_id": "client_456",
     "service_name": "web_browser"
   }
   ```
   - Click "Invoke"

5. **Test StreamNotifications**
   - Method: `notification.NotificationService/StreamNotifications`
   - Message:
   ```json
   {
     "connection_id": "client_456"
   }
   ```
   - Click "Invoke" - This will keep stream open to receive notifications

## Sending Notifications Programmatically

To send notifications to clients, you need to call the handler methods from within your Go code or create an HTTP gateway.

### Example: Send Notification to All Client Devices

```go
import (
    "time"
    "grpcon/models"
)

// Get the notification server (from your server instance)
notificationServer := server.GetNotificationServer()

// Create notification
notification := &models.NotificationData{
    ID:          "notif_001",
    ClientID:    "user123",  // All devices of user123 will receive this
    Title:       "New Message",
    Message:     "You have a new notification!",
    ServiceName: "notification_service",
    Timestamp:   time.Now().Unix(),
}

// Send to all devices of this client
err := notificationServer.SendNotificationToClient(notification)
```

### Example: Broadcast to All Clients

```go
notification := &models.NotificationData{
    ID:          "broadcast_001",
    ClientID:    "",  // Not used in broadcast
    Title:       "System Announcement",
    Message:     "Maintenance scheduled at 10 PM",
    ServiceName: "admin_service",
    Timestamp:   time.Now().Unix(),
}

// Broadcast to all connected clients and their devices
notificationServer.BroadcastNotification(notification)
```

## Creating an HTTP Gateway for Testing

Create a simple HTTP endpoint to trigger notifications:

```go
// Add to main.go or create a separate HTTP server

import (
    "encoding/json"
    "net/http"
    "time"
)

func setupHTTPGateway(notifServer *handlers.NotificationServer) {
    http.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
        var req struct {
            ClientID string `json:"client_id"`
            Title    string `json:"title"`
            Message  string `json:"message"`
        }
        
        json.NewDecoder(r.Body).Decode(&req)
        
        notification := &models.NotificationData{
            ID:          fmt.Sprintf("notif_%d", time.Now().Unix()),
            ClientID:    req.ClientID,
            Title:       req.Title,
            Message:     req.Message,
            ServiceName: "http_gateway",
            Timestamp:   time.Now().Unix(),
        }
        
        err := notifServer.SendNotificationToClient(notification)
        if err != nil {
            w.WriteHeader(http.StatusInternalServerError)
            json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
            return
        }
        
        json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
    })
    
    http.ListenAndServe(":8080", nil)
}
```

Then test with curl or Postman:

```bash
curl -X POST http://localhost:8080/send \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "user123",
    "title": "Hello",
    "message": "Test notification"
  }'
```

## Connection Statistics

You can get connection statistics programmatically:

```go
stats := notificationServer.GetConnectionStats()
// Returns:
// {
//   "total_clients": 5,
//   "total_devices": 12,
//   "client_ids": ["user123", "user456", ...]
// }
```

## Example: Complete Testing Workflow

1. **Start the server:**
   ```bash
   go run main.go
   ```

2. **Register Device 1 for Client "alice":**
   ```bash
   grpcurl -plaintext -d '{"connection_id": "alice", "service_name": "phone"}' localhost:50051 notification.NotificationService/AddConnection
   ```

3. **Register Device 2 for Client "alice":**
   ```bash
   grpcurl -plaintext -d '{"connection_id": "alice", "service_name": "laptop"}' localhost:50051 notification.NotificationService/AddConnection
   ```

4. **Start streaming on Device 1 (in a new terminal):**
   ```bash
   grpcurl -plaintext -d '{"connection_id": "alice"}' localhost:50051 notification.NotificationService/StreamNotifications
   ```

5. **Start streaming on Device 2 (in a new terminal):**
   ```bash
   grpcurl -plaintext -d '{"connection_id": "alice"}' localhost:50051 notification.NotificationService/StreamNotifications
   ```

6. **Send a notification** (requires HTTP gateway or programmatic call):
   - Both Device 1 and Device 2 will receive the notification
   - Check the server logs to see notification delivery stats

## Data Storage Recommendations

For storing connection IDs and tracking metrics:

1. **In-Memory (Current):**
   - Fast access with `sync.RWMutex`
   - Great for development and testing
   - Lost on server restart

2. **Redis (Production):**
   - Persist connection metadata
   - Distributed systems support
   - TTL for automatic cleanup

3. **PostgreSQL/MongoDB (Long-term Tracking):**
   - Store historical connection data
   - Analytics on notification patterns
   - User device history

## Notes

- Current implementation uses in-memory storage (lost on restart)
- For production, add authentication and authorization
- Consider adding rate limiting for notifications
- Implement connection cleanup for inactive devices
- Add health check endpoints

## License

MIT
