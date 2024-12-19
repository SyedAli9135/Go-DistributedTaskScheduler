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

// AtomicUpdateTaskOwnership attempts to atomically update the task's ownership.
// It succeeds only if the task's Owner field is empty (i.e., not claimed).
func (s *EtcdStore) AtomicUpdateTaskOwnership(task common.Task) error {
	// Use a context with timeout to avoid blocking indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := "tasks/" + task.ID

	// Fetch the current task from Etcd
	resp, err := s.client.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to fetch task: %v", err)
	}

	// If the task doesn't exist, return an error
	if len(resp.Kvs) == 0 {
		return fmt.Errorf("task %s not found", task.ID)
	}

	// Unmarshal the task from Etcd
	currentTask := &common.Task{}
	if err := json.Unmarshal(resp.Kvs[0].Value, currentTask); err != nil {
		return fmt.Errorf("failed to unmarshal task: %v", err)
	}

	// If the task already has an owner, return an error (it has already been claimed)
	if currentTask.Owner != "" {
		return fmt.Errorf("task %s is already claimed by worker %s", task.ID, currentTask.Owner)
	}

	// Marshal the updated task (with ownership assigned to the worker)
	taskData, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %v", err)
	}

	// Use Etcd's Transaction (Txn) to atomically update the task
	_, err = s.client.Txn(ctx).
		If(clientv3.Compare(clientv3.Value(key), "=", "")).
		Then(clientv3.OpPut(key, string(taskData))).
		Else(clientv3.OpGet(key)).
		Commit()

	if err != nil {
		return fmt.Errorf("failed to atomically update task: %v", err)
	}

	return nil
}

// AcquireLock tries to acquire a lock for the given task.
// Returns an error if the lock could not be acquired.
func (s *EtcdStore) AcquireLock(taskID string, workerID string) error {
	// Create a context with timeout to ensure our operation doesn't hang forever
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a lease - this is like a time-limited contract
	// The lease will automatically expire after 30 seconds
	// This prevents deadlocks if a worker crashes while holding a lock
	lease, err := s.client.Grant(ctx, 30) // 30 second TTL
	if err != nil {
		return fmt.Errorf("failed to create lease: %w", err)
	}

	// Create the key that will be used in etcd to represent this lock
	// For example, if taskID is "task123", lockKey will be "locks/task123"
	lockKey := fmt.Sprintf("locks/%s", taskID)

	// Try to acquire lock using a transaction to ensure our operations are automic
	txn := s.client.Txn(ctx).
		// Check if the key doesn't exist
		If(clientv3.Compare(clientv3.CreateRevision(lockKey), "=", 0)).
		// If it doesn't exist, create it with our lease
		Then(clientv3.OpPut(lockKey, workerID, clientv3.WithLease(lease.ID))).
		// If it exists, get its value
		Else(clientv3.OpGet(lockKey))

	// Try to execute the transaction
	txnResp, err := txn.Commit()
	if err != nil {
		return fmt.Errorf("failed to execute transaction: %w", err)
	}

	// If the transaction's condition was false (key exists), it means someone else has the lock
	if !txnResp.Succeeded {
		return fmt.Errorf("lock already held by another worker")
	}

	return nil
}

// Releases the acquired lock by etcd
func (s *EtcdStore) ReleaseLock(taskID string) error {
	// Create a context with timeout to ensure our operation doesn't hang
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create the lock key - same format as in AcquireLock
	lockKey := fmt.Sprintf("locks/%s", taskID)

	// Get the current lock value and lease
	resp, err := s.client.Get(ctx, lockKey, clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("failed to get lock info: %w", err)
	}

	// If the key doesn't exist, nothing to release
	if len(resp.Kvs) == 0 {
		return nil
	}

	// Get the lease ID from the key
	leaseID := clientv3.LeaseID(resp.Kvs[0].Lease)

	// Delete the lock key
	_, err = s.client.Delete(ctx, lockKey)
	if err != nil {
		return fmt.Errorf("failed to delete lock: %w", err)
	}

	// Revoke the lease if it exists
	if leaseID != 0 {
		_, err = s.client.Revoke(ctx, leaseID)
		if err != nil {
			return fmt.Errorf("failed to revoke lease: %w", err)
		}
	}

	return nil
}
