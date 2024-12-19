package common

import "time"

// Task represents a task in the system
type Task struct {
	ID         string    `json:"id"`          // Unique identifier
	Name       string    `json:"name"`        // Task name
	Status     string    `json:"status"`      // Current status (e.g., pending, running, completed)
	CreatedAt  time.Time `json:"created_at"`  // Timestamp when the task was created
	ExecuteAt  time.Time `json:"execute_at"`  // Scheduled execution time
	RetryCount int       `json:"retry_count"` // Number of retries allowed
}
