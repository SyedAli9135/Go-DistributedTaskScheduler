package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"distributed-task-scheduler/internal/common"
	"distributed-task-scheduler/internal/storage"
)

// Global store instance for interacting with etcd
var store *storage.EtcdStore

// InitStore initializes the etcd store with default configurations
// This function sets up the etcd store instance globally.
func InitStore() {
	var err error
	store, err = storage.NewEtcdStore([]string{"localhost:2379"}, 5*time.Second)
	if err != nil {
		panic("Failed to initialize etcd store: " + err.Error())
	}
}

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

// createTask handles the creation of a new task
// Request Body: JSON with "name" and "execute_at" fields.
// Response: Returns the created task in JSON format.
func createTask(c *gin.Context) {
	var input struct {
		Name      string    `json:"name" binding:"required"`
		ExecuteAt time.Time `json:"execute_at"`
	}

	// Bind JSON payload to the input struct
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create a new task with a unique ID
	task := common.Task{
		ID:         uuid.New().String(),
		Name:       input.Name,
		Status:     "pending",
		CreatedAt:  time.Now(),
		ExecuteAt:  input.ExecuteAt,
		RetryCount: 0,
	}

	// Save the task to etcd
	if err := store.AddTask(task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Respond with the created task
	c.JSON(http.StatusCreated, task)
}

// getTask retrieves a task by its ID
// URL Parameter: "id" - The ID of the task to retrieve.
// Response: Returns the task details in JSON format.
func getTask(c *gin.Context) {
	id := c.Param("id")

	// Fetch the task from etcd
	task, err := store.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Respond with the task details
	c.JSON(http.StatusOK, task)
}

// listTasks retrieves all tasks stored in etcd
// Response: Returns a list of all tasks in JSON format.
func listTasks(c *gin.Context) {
	// Fetch all tasks from etcd
	tasks, err := store.ListTasks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Respond with the list of tasks
	c.JSON(http.StatusOK, tasks)
}
