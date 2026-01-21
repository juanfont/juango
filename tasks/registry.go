// Package tasks provides Asynq background task helpers.
package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
)

// Client wraps an Asynq client for enqueuing tasks.
type Client struct {
	client *asynq.Client
}

// NewClient creates a new task client.
func NewClient(redisAddr, redisPassword string, redisDB int) *Client {
	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})
	return &Client{client: client}
}

// Close closes the task client.
func (c *Client) Close() error {
	return c.client.Close()
}

// Enqueue enqueues a task with the given type and payload.
func (c *Client) Enqueue(taskType string, payload interface{}, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling task payload: %w", err)
	}

	task := asynq.NewTask(taskType, data)
	info, err := c.client.Enqueue(task, opts...)
	if err != nil {
		return nil, fmt.Errorf("enqueuing task: %w", err)
	}

	log.Info().
		Str("task_type", taskType).
		Str("task_id", info.ID).
		Msg("Task enqueued")

	return info, nil
}

// EnqueueIn enqueues a task to be processed after the specified delay.
func (c *Client) EnqueueIn(taskType string, payload interface{}, delay time.Duration, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	opts = append(opts, asynq.ProcessIn(delay))
	return c.Enqueue(taskType, payload, opts...)
}

// EnqueueAt enqueues a task to be processed at the specified time.
func (c *Client) EnqueueAt(taskType string, payload interface{}, processAt time.Time, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	opts = append(opts, asynq.ProcessAt(processAt))
	return c.Enqueue(taskType, payload, opts...)
}

// Server wraps an Asynq server for processing tasks.
type Server struct {
	server *asynq.Server
	mux    *asynq.ServeMux
}

// ServerConfig holds configuration for the task server.
type ServerConfig struct {
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	Concurrency   int
	Queues        map[string]int // Queue name -> priority
}

// DefaultServerConfig returns a default server configuration.
func DefaultServerConfig(redisAddr, redisPassword string, redisDB int) *ServerConfig {
	return &ServerConfig{
		RedisAddr:     redisAddr,
		RedisPassword: redisPassword,
		RedisDB:       redisDB,
		Concurrency:   10,
		Queues: map[string]int{
			"critical": 6,
			"default":  3,
			"low":      1,
		},
	}
}

// NewServer creates a new task server.
func NewServer(cfg *ServerConfig) *Server {
	server := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		},
		asynq.Config{
			Concurrency: cfg.Concurrency,
			Queues:      cfg.Queues,
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				log.Error().
					Err(err).
					Str("task_type", task.Type()).
					Bytes("payload", task.Payload()).
					Msg("Task failed")
			}),
		},
	)

	return &Server{
		server: server,
		mux:    asynq.NewServeMux(),
	}
}

// HandleFunc registers a handler function for the given task type.
func (s *Server) HandleFunc(taskType string, handler func(context.Context, *asynq.Task) error) {
	s.mux.HandleFunc(taskType, handler)
	log.Debug().Str("task_type", taskType).Msg("Registered task handler")
}

// Handle registers a handler for the given task type.
func (s *Server) Handle(taskType string, handler asynq.Handler) {
	s.mux.Handle(taskType, handler)
	log.Debug().Str("task_type", taskType).Msg("Registered task handler")
}

// Run starts the server and blocks until shutdown.
func (s *Server) Run() error {
	log.Info().Msg("Starting task server")
	return s.server.Run(s.mux)
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() {
	log.Info().Msg("Shutting down task server")
	s.server.Shutdown()
}

// TaskHandler is an interface for task handlers with automatic JSON unmarshaling.
type TaskHandler[T any] struct {
	handler func(context.Context, T) error
}

// NewTaskHandler creates a new typed task handler.
func NewTaskHandler[T any](handler func(context.Context, T) error) *TaskHandler[T] {
	return &TaskHandler[T]{handler: handler}
}

// ProcessTask implements asynq.Handler.
func (h *TaskHandler[T]) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload T
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshaling task payload: %w", err)
	}
	return h.handler(ctx, payload)
}

// Common task type constants.
const (
	TaskTypeEmailNotification = "email:notification"
	TaskTypeSyncData          = "sync:data"
	TaskTypeCleanup           = "maintenance:cleanup"
)

// EmailNotificationPayload is the payload for email notification tasks.
type EmailNotificationPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// SyncDataPayload is the payload for data sync tasks.
type SyncDataPayload struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}
