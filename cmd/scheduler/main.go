package main

import (
	"context"
	"distributed-task-scheduler/api"
	"distributed-task-scheduler/internal/worker"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	// Initializes etcd storage
	store := api.InitStore()

	// Initialize gin router
	router := gin.Default()

	// Setup api routes
	api.SetupRoutes(router)

	// Start the API server in a separate goroutine
	go func() {
		port := ":8080"
		log.Printf("Starting Task Scheduler on %s...", port)
		if err := router.Run(port); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Context for graceful shutdown of workers
	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	// Create and start worker nodes
	worker1 := worker.NewWorker("worker-1", store, 5*time.Second, 3)
	worker2 := worker.NewWorker("worker-2", store, 5*time.Second, 3)

	go worker1.Start(ctx)
	go worker2.Start(ctx)

	// Hande OS signals for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	<-signalChan
	log.Println("Shutting down gracefully...")

	// Stop workers
	stop()

}
