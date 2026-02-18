# Architecture Overview

## Connection Hierarchy

```
┌─────────────────────────────────────────────────────────────┐
│               ConnectionManager (Thread-Safe)                │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  ClientGroup: "user123"                              │  │
│  │  ┌────────────────────────────────────────────────┐  │  │
│  │  │  Device: "mobile_app"                          │  │  │
│  │  │  UniqueID: user123_mobile_app                  │  │  │
│  │  │  Connected: 2026-02-18 10:30:00                │  │  │
│  │  │  Last Notification: 2026-02-18 10:35:00        │  │  │
│  │  │  Notification Count: 15                        │  │  │
│  │  │  Stream: Active ✓                              │  │  │
│  │  └────────────────────────────────────────────────┘  │  │
│  │  ┌────────────────────────────────────────────────┐  │  │
│  │  │  Device: "web_browser"                         │  │  │
│  │  │  UniqueID: user123_web_browser                 │  │  │
│  │  │  Connected: 2026-02-18 10:32:00                │  │  │
│  │  │  Last Notification: 2026-02-18 10:35:00        │  │  │
│  │  │  Notification Count: 12                        │  │  │
│  │  │  Stream: Active ✓                              │  │  │
│  │  └────────────────────────────────────────────────┘  │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  ClientGroup: "user456"                              │  │
│  │  ┌────────────────────────────────────────────────┐  │  │
│  │  │  Device: "tablet"                              │  │  │
│  │  │  UniqueID: user456_tablet                      │  │  │
│  │  │  Connected: 2026-02-18 11:00:00                │  │  │
│  │  │  Last Notification: Never                      │  │  │
│  │  │  Notification Count: 0                         │  │  │
│  │  │  Stream: Active ✓                              │  │  │
│  │  └────────────────────────────────────────────────┘  │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## Data Flow

### 1. Device Connection Flow

```
Client Device                  gRPC Server                ConnectionManager
     │                              │                              │
     ├──AddConnection──────────────>│                              │
     │  {client_id, device_id}      │                              │
     │                              ├──RegisterDevice──────────────>│
     │                              │                              │
     │                              │                    ┌─────────┴──────────┐
     │                              │                    │ Create Connection  │
     │                              │                    │ Add to ClientGroup │
     │                              │                    └─────────┬──────────┘
     │<──ConnectionResponse─────────┤                              │
     │  {success, unique_id}        │<──────────────────────────────┘
     │                              │
     ├──StreamNotifications────────>│
     │  {connection_id}             │
     │                              ├──AttachStream────────────────>│
     │                              │                    ┌─────────┴──────────┐
     │<═════Stream Established══════│                    │ Update conn.Stream │
     │                              │                    │ Set IsActive=true  │
     │                              │                    └────────────────────┘
     │                              │
```

### 2. Notification Flow (Target Specific Client)

```
Service/API                    NotificationServer         ConnectionManager
     │                              │                              │
     ├──SendNotification───────────>│                              │
     │  {client_id, title, msg}     │                              │
     │                              ├──GetClientGroup─────────────>│
     │                              │                              │
     │                              │                    ┌─────────┴──────────┐
     │                              │                    │ Find ClientGroup   │
     │                              │                    │ Get all devices    │
     │                              │                    └─────────┬──────────┘
     │                              │<─────────────────────────────┘
     │                              │  [device1, device2, ...]
     │                              │
     │                     ┌────────┴─────────┐
     │                     │ For each device: │
     │                     │  - Send via gRPC │
     │                     │  - Update metrics│
     │                     └────────┬─────────┘
     │                              │
Device1<══════Notification══════════┤
Device2<══════Notification══════════┤
     │                              │
     │<──Response────────────────────┤
     │  {success, count}            │
```

### 3. Broadcast Flow

```
Service/API                    NotificationServer         ConnectionManager
     │                              │                              │
     ├──Broadcast───────────────────>│                              │
     │  {title, message}            │                              │
     │                              ├──GetAllClientIDs────────────>│
     │                              │<─────────────────────────────┤
     │                              │  [client1, client2, ...]
     │                              │
     │                     ┌────────┴──────────┐
     │                     │ For each client:  │
     │                     │  Get devices      │
     │                     │  Send to all      │
     │                     └────────┬──────────┘
     │                              │
All Devices<════Notification════════┤
     │                              │
```

## Data Structures

### Connection
```go
type Connection struct {
    UniqueID             string    // "client_id_device_id"
    ClientID             string    // "user123"
    DeviceID             string    // "mobile_app"
    ServiceName          string    // "notification_service"
    Stream               gRPC_Stream
    ConnectedAt          time.Time
    LastNotificationAt   time.Time
    NotificationCount    int
    IsActive             bool
}
```

### ClientGroup
```go
type ClientGroup struct {
    ClientID  string
    Devices   map[string]*Connection  // key: device_id
    mu        sync.RWMutex
}
```

### ConnectionManager
```go
type ConnectionManager struct {
    clients   map[string]*ClientGroup  // key: client_id
    mu        sync.RWMutex
}
```

## Thread Safety

All operations on shared data use Read/Write mutexes:

- **Read operations** (queries): `RLock()` / `RUnlock()`
  - GetConnection
  - GetAllDevices
  - GetStats

- **Write operations** (mutations): `Lock()` / `Unlock()`
  - AddConnection
  - RemoveConnection
  - AttachStream

This allows multiple concurrent reads while ensuring write exclusivity.

## Use Cases

### Use Case 1: Multi-Device User Notifications
```
User "alice" has 3 devices: phone, laptop, tablet
When notification is sent to "alice":
  → All 3 devices receive the notification
  → Each device's metrics are updated independently
  → If one device is offline, notification still sent to others
```

### Use Case 2: Presence Tracking
```
Track how long each device has been connected:
  → conn.GetUptime() returns duration since ConnectedAt
  → Useful for analytics and billing
```

### Use Case 3: Notification History
```
Each connection tracks:
  → NotificationCount: Total notifications received
  → LastNotificationAt: When last notification was received
  → Can be used to determine inactive devices
```

## Scaling Considerations

### Current (In-Memory)
- Fast access: O(1) lookups
- Limited to single server instance
- Lost on restart

### Redis-Based
```
Key Structure:
  clients:{client_id} → Set of device_ids
  device:{client_id}:{device_id} → Hash (ConnectedAt, NotificationCount, etc.)
  stats:total_clients → Counter
  stats:total_devices → Counter
```

### Database-Backed (PostgreSQL)
```sql
CREATE TABLE connections (
    unique_id VARCHAR PRIMARY KEY,
    client_id VARCHAR NOT NULL,
    device_id VARCHAR NOT NULL,
    service_name VARCHAR,
    connected_at TIMESTAMP,
    last_notification_at TIMESTAMP,
    notification_count INTEGER,
    is_active BOOLEAN,
    INDEX idx_client_id (client_id)
);
```
