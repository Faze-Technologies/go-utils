package cache

import (
	"context"
	"errors"
	"fmt"
	"github.com/Faze-Technologies/go-utils/config"
	"github.com/Faze-Technologies/go-utils/constants"
	"github.com/Faze-Technologies/go-utils/utils"
	"github.com/goccy/go-json"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"time"
)

type Cache struct {
	rDB *redis.Client
}

func NewCache() *Cache {
	logger := utils.GetLogger()
	client := redis.NewClient(&redis.Options{
		Addr:     config.GetString("redis.address"),
		Password: config.GetString("redis.password"),
		DB:       config.GetInt("redis.db"),
		PoolSize: config.GetInt("redis.poolSize"),
	})
	pong, err := client.Ping(context.Background()).Result()
	if err != nil {
		logger.Panic(fmt.Sprintf("Failed to connect to Redis: %v", err))
	}
	logger.Info("Connected to Redis", zap.String("PING", pong))
	return &Cache{
		rDB: client,
	}
}

func (cache Cache) SetJson(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	result := cache.rDB.Set(ctx, key, bytes, expiration)
	logger := utils.GetLogger()
	logger.Debug("SETTING IN REDIS", zap.String("key", key), zap.String("exp", expiration.String()))
	return result.Err()
}

func (cache Cache) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	result := cache.rDB.Set(ctx, key, value, expiration)
	logger := utils.GetLogger()
	logger.Debug("SETTING IN REDIS",
		zap.String("key", key),
		zap.String("value", value),
		zap.String("exp", expiration.String()),
	)
	return result.Err()
}

func (cache Cache) Incr(ctx context.Context, key string, expiration time.Duration) (int64, error) {
	count, err := cache.rDB.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if count == 1 && expiration != 0 {
		cache.rDB.Expire(ctx, key, expiration)
	}
	logger := utils.GetLogger()
	logger.Debug("INCREMENTING IN REDIS", zap.String("key", key), zap.Int64("count", count))
	return count, nil
}

func (cache Cache) TTL(ctx context.Context, key string) (time.Duration, error) {
	return cache.rDB.TTL(ctx, key).Result()
}

func (cache Cache) Get(ctx context.Context, key string) (string, error) {
	result, err := cache.rDB.Get(ctx, key).Result()
	switch {
	case errors.Is(err, redis.Nil):
		return "", fmt.Errorf(string(constants.KeyNotFoundError))
	case err != nil:
		return "", err
	case result == "":
		return "", fmt.Errorf(string(constants.KeyNotFoundError))
	}
	logger := utils.GetLogger()
	logger.Debug("GETTING FROM REDIS", zap.String("key", key))
	return result, nil
}

func (cache Cache) GetJSON(ctx context.Context, key string, value interface{}) error {
	result := cache.rDB.Get(ctx, key)
	storedBytes, err := result.Bytes()
	if err != nil {
		return err
	}
	logger := utils.GetLogger()
	logger.Debug("GETTING FROM REDIS", zap.String("key", key))
	return json.Unmarshal(storedBytes, &value)
}

func (cache Cache) Delete(ctx context.Context, key string) error {
	return cache.rDB.Del(ctx, key).Err()
}

func (cache Cache) DeleteWithPattern(ctx context.Context, pattern string) error {
	logger := utils.GetLogger()
	logger.Debug("DELETING KEYS MATCHING PATTERN", zap.String("pattern", pattern))
	result, err := cache.rDB.Keys(ctx, pattern).Result()
	if err != nil {
		return err
	}
	return cache.rDB.Del(ctx, result...).Err()
}
