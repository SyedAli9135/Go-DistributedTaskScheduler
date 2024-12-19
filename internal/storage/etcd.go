package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"distributed-task-scheduler/internal/common"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdStore provides etcd-backed storage for tasks
// This struct encapsulates the etcd client and timeout configurations.
type EtcdStore struct {
	client  *clientv3.Client // The etcd client used for communication
	timeout time.Duration    // Timeout for etcd operations
}

// NewEtcdStore initializes a new EtcdStore
// Accepts etcd endpoints and a timeout duration for client operations.
func NewEtcdStore(endpoints []string, timeout time.Duration) (*EtcdStore, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints, // etcd cluster endpoints
		DialTimeout: timeout,   // Timeout for connecting to the etcd cluster
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to etcd: %w", err)
	}

	return &EtcdStore{
		client:  cli,
		timeout: timeout,
	}, nil
}

// AddTask adds a new task to etcd
// The task is serialized to JSON and stored under the key "tasks/<task.ID>".
func (s *EtcdStore) AddTask(task common.Task) error {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	// Key format: "tasks/<task.ID>"
	taskKey := fmt.Sprintf("tasks/%s", task.ID)

	// Serialize the task to JSON
	taskData, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	// Store the task in etcd
	_, err = s.client.Put(ctx, taskKey, string(taskData))
	return err
}

// GetTask retrieves a task by ID from etcd
// The key "tasks/<id>" is used to fetch the task JSON and deserialize it.
func (s *EtcdStore) GetTask(id string) (common.Task, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	// Key format: "tasks/<id>"
	taskKey := fmt.Sprintf("tasks/%s", id)

	// Fetch the task from etcd
	resp, err := s.client.Get(ctx, taskKey)
	if err != nil {
		return common.Task{}, fmt.Errorf("failed to get task: %w", err)
	}

	// If no data is found, return an error
	if len(resp.Kvs) == 0 {
		return common.Task{}, fmt.Errorf("task not found")
	}

	// Deserialize the task JSON
	var task common.Task
	if err := json.Unmarshal(resp.Kvs[0].Value, &task); err != nil {
		return common.Task{}, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	return task, nil
}

// ListTasks lists all tasks stored in etcd
// This retrieves all keys with the "tasks/" prefix and deserializes their values.
func (s *EtcdStore) ListTasks() ([]common.Task, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	// Fetch all tasks with the "tasks/" prefix
	resp, err := s.client.Get(ctx, "tasks/", clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	var tasks []common.Task
	// Deserialize each task JSON and append to the list
	for _, kv := range resp.Kvs {
		var task common.Task
		if err := json.Unmarshal(kv.Value, &task); err != nil {
			return nil, fmt.Errorf("failed to unmarshal task: %w", err)
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}
