package server

import (
	"context"
	"encoding/json"
	"time"

	"github.com/kastoras/images-api/internal/config"
	"github.com/redis/go-redis/v9"
)

const jobsQueue = "pending_jobs"

type Cache struct {
	client *redis.Client
}

func initCache(cfg *config.Config) *Cache {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisURL,
		Password: cfg.RedisPassword,
	})
	return &Cache{client: client}
}

func (c *Cache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *Cache) Enqueue(ctx context.Context, jobID string) error {
	return c.client.RPush(ctx, jobsQueue, jobID).Err()
}

func (c *Cache) Dequeue(ctx context.Context) (string, error) {
	result, err := c.client.BLPop(ctx, 0, jobsQueue).Result()
	if err != nil {
		return "", err
	}
	return result[1], nil
}

func (c *Cache) QueueDepth(ctx context.Context) (int64, error) {
	return c.client.LLen(ctx, jobsQueue).Result()
}

func (c *Cache) SetJobMeta(ctx context.Context, jobID string, meta any, ttl time.Duration) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, "job:"+jobID, data, ttl).Err()
}

func (c *Cache) GetJobMeta(ctx context.Context, jobID string, dest any) error {
	data, err := c.client.Get(ctx, "job:"+jobID).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func (c *Cache) Close() error {
	return c.client.Close()
}
