package api

import (
	"distributed-task-scheduler/internal/common"
	"distributed-task-scheduler/internal/storage"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Global store (in-memory for now)
var store = storage.NewMemoryStore()

// SetupRoutes initializes the API routes
func SetupRoutes(router *gin.Engine) {
	// Health Check
	router.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{
			"status": "OK",
		})
	})

	// Task management routes
	taskGroup := router.Group("/tasks")
	{
		taskGroup.POST("/", createTask) // Create a new task
		taskGroup.GET("/:id", getTask)  // Get task details
		taskGroup.GET("/", listTasks)   // List all tasks
	}
}

// Example handlers (to be implemented)
func createTask(c *gin.Context) {
	var input struct {
		Name      string    `json:"name" binding:"required"`
		ExecuteAt time.Time `json:"execute_at"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task := common.Task{
		ID:         uuid.New().String(),
		Name:       input.Name,
		Status:     "pending",
		CreatedAt:  time.Now(),
		ExecuteAt:  input.ExecuteAt,
		RetryCount: 0,
	}

	store.AddTask(task)
	c.JSON(http.StatusCreated, task)
}

func getTask(c *gin.Context) {
	id := c.Param("id")
	task, err := store.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

func listTasks(c *gin.Context) {
	tasks := store.ListTasks()
	c.JSON(http.StatusOK, tasks)
}
