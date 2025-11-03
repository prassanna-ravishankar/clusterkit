package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Hostname  string    `json:"hostname"`
	Uptime    string    `json:"uptime"`
}

type MessageResponse struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

var startTime = time.Now()

func main() {
	// Get port from environment or default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Setup routes
	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/api/health", handleHealth)
	http.HandleFunc("/api/message", handleMessage)

	// Start server
	log.Printf("ClusterKit Demo API starting on port %s", port)
	log.Printf("Ready to handle requests...")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>ClusterKit Demo API</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            max-width: 800px;
            margin: 50px auto;
            padding: 20px;
        }
        h1 { color: #667eea; }
        .endpoint {
            background: #f4f4f4;
            padding: 10px;
            margin: 10px 0;
            border-left: 4px solid #667eea;
        }
        code {
            background: #eee;
            padding: 2px 6px;
            border-radius: 3px;
        }
    </style>
</head>
<body>
    <h1>🚀 ClusterKit Demo API</h1>
    <p>This is a sample API demonstrating ClusterKit's serverless capabilities.</p>

    <h2>Available Endpoints</h2>

    <div class="endpoint">
        <strong>GET /health</strong> or <strong>GET /api/health</strong><br>
        Returns API health status and metadata
    </div>

    <div class="endpoint">
        <strong>GET /api/message</strong><br>
        Returns a demo message
    </div>

    <h2>Features</h2>
    <ul>
        <li>⚡ Scales to zero when idle</li>
        <li>📈 Auto-scales based on load</li>
        <li>🔄 Load balanced across pods</li>
        <li>📊 Health check endpoints</li>
    </ul>

    <p><em>Powered by Knative Serving on GKE Autopilot</em></p>
</body>
</html>
	`)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()

	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Hostname:  hostname,
		Uptime:    time.Since(startTime).String(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	json.NewEncoder(w).Encode(response)
	log.Printf("Health check: %s from %s", hostname, r.RemoteAddr)
}

func handleMessage(w http.ResponseWriter, r *http.Request) {
	response := MessageResponse{
		Message:   "Hello from ClusterKit! This API scales automatically based on demand.",
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	json.NewEncoder(w).Encode(response)
	log.Printf("Message request from %s", r.RemoteAddr)
}
