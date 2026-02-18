package main

import (
	"encoding/json"
	"fmt"
	"grpcon/handlers"
	"grpcon/models"
	"log"
	"net/http"
	"time"
)

func main() {
	// Create notification server
	notifServer := handlers.NewNotificationServer()

	log.Println("Starting HTTP server on :8080")
	setupHTTPGateway(notifServer)
}

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
