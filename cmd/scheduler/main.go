package main

import (
	"distributed-task-scheduler/api"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize gin router
	router := gin.Default()

	// Setup api routes
	api.SetupRoutes(router)

	// Start the server
	port := ":8080"
	log.Printf("Starting Task Scheduler on %s...", port)
	if err := router.Run(port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
