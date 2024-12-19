package worker

import (
	"context"
	"log"
	"time"

	"distributed-task-scheduler/internal/common"
	"distributed-task-scheduler/internal/storage"
)

// Worker represents a task-processing worker
type Worker struct {
	ID           string             // Unique identifier for the worker
	Store        *storage.EtcdStore // Storage interface to fetch and update tasks
	PollInterval time.Duration      // Polling interval for task retrieval
	MaxRetries   int                // Maximum retry attempts for a task
}

// NewWorker initializes and returns a Worker instance
func NewWorker(id string, store *storage.EtcdStore, pollInterval time.Duration, maxRetries int) *Worker {
	return &Worker{
		ID:           id,
		Store:        store,
		PollInterval: pollInterval,
		MaxRetries:   maxRetries,
	}
}

// Start begins the worker's task processing loop
func (w *Worker) Start(ctx context.Context) {
	log.Printf("Worker %s started", w.ID)

	ticker := time.NewTicker(w.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.processTasks()
		case <-ctx.Done():
			log.Printf("Worker %s stopping", w.ID)
			return
		}
	}
}

// processTasks fetches and processes tasks that are ready
func (w *Worker) processTasks() {
	tasks, err := w.Store.ListTasks()
	if err != nil {
		log.Printf("Worker %s: failed to list tasks: %v", w.ID, err)
		return
	}

	for _, task := range tasks {
		if task.Status == "pending" && task.ExecuteAt.Before(time.Now()) {
			if task.Owner == "" {
				log.Printf("Worker %s: attempting to claim task %s", w.ID, task.ID)

				// Try to acquire the lock
				err := w.Store.AcquireLock(task.ID, w.ID)
				if err != nil {
					// If we couldn't get the lock, just skip this task
					log.Printf("Worker %s: could not acquire lock for task %s: %v", w.ID, task.ID, err)
					continue
				}

				// If we got the lock, process the task
				w.claimAndProcessTask(task)

				// After processing, release the lock
				if err := w.Store.ReleaseLock(task.ID); err != nil {
					log.Printf("Worker %s: failed to release lock for task %s: %v", w.ID, task.ID, err)
				}
			}
		}
	}
}

// claimAndProcessTask claims the ownership and processes task
func (w *Worker) claimAndProcessTask(task common.Task) {
	// Claim the task by assigning ownership
	task.Owner = w.ID
	task.Status = "processing"
	if err := w.Store.AtomicUpdateTaskOwnership(task); err != nil {
		log.Printf("Worker %s: failed to claim task %s: %v", w.ID, task.ID, err)
		return
	}

	// Proceed with task processing
	w.processTask(task)
}

// processTask transitions the task through processing and completion
func (w *Worker) processTask(task common.Task) {
	log.Printf("Worker %s: processing task %s", w.ID, task.ID)

	// Update task status to processing
	task.Status = "processing"
	if err := w.Store.AddTask(task); err != nil {
		log.Printf("Worker %s: failed to update task %s to processing: %v", w.ID, task.ID, err)
		return
	}

	// Simulate task execution
	err := w.executeTask(task)
	if err != nil {
		w.handleTaskFailure(task, err)
		return
	}

	// Mark the task as completed
	task.Status = "completed"
	if err := w.Store.AddTask(task); err != nil {
		log.Printf("Worker %s: failed to update task %s to completed: %v", w.ID, task.ID, err)
	}
	log.Printf("Worker %s: successfully completed task %s", w.ID, task.ID)
}

// executeTask simulates the execution of a task (replace this with real logic)
func (w *Worker) executeTask(task common.Task) error {
	log.Printf("Worker %s: executing task %s", w.ID, task.ID)
	time.Sleep(2 * time.Second) // Simulate some work being done
	return nil                  // Replace with real execution logic
}

// handleTaskFailure handles task retries and failure marking
func (w *Worker) handleTaskFailure(task common.Task, err error) {
	log.Printf("Worker %s: task %s failed: %v", w.ID, task.ID, err)

	task.RetryCount++
	if task.RetryCount > w.MaxRetries {
		task.Status = "failed"
		log.Printf("Worker %s: task %s marked as failed after %d retries", w.ID, task.ID, w.MaxRetries)
	} else {
		task.Status = "pending" // Reset status for retry
		log.Printf("Worker %s: task %s will be retried (attempt %d)", w.ID, task.ID, task.RetryCount)
	}

	// Update task state in etcd
	if err := w.Store.AddTask(task); err != nil {
		log.Printf("Worker %s: failed to update task %s: %v", w.ID, task.ID, err)
	}
}
