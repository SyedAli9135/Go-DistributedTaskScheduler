package storage

import (
	"distributed-task-scheduler/internal/common"
	"errors"
	"sync"
)

// MemoryStore provides an in-memory storage for tasks
type MemoryStore struct {
	tasks map[string]common.Task
	mu    sync.RWMutex
}

// NewMemoryStore initializes a new MemoryStore
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		tasks: make(map[string]common.Task),
	}
}

// AddTask adds a new task to the store
func (s *MemoryStore) AddTask(task common.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[task.ID] = task
}

// GetTasks retrieves a task by ID
func (s *MemoryStore) GetTask(id string) (common.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, exists := s.tasks[id]
	if !exists {
		return common.Task{}, errors.New("task not found")
	}
	return task, nil
}

// ListTasks lists all tasks
func (s *MemoryStore) ListTasks() []common.Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	tasks := make([]common.Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}
