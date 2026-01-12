package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Faze-Technologies/go-utils/config"
	"github.com/Faze-Technologies/go-utils/logs"
	"github.com/Faze-Technologies/go-utils/request"
	"github.com/goccy/go-json"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Cache struct {
	rDB *redis.Client
}

func NewCache() *Cache {
	logger := logs.GetLogger()
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
	logger := logs.GetLogger()
	logger.Debug("SETTING IN REDIS", zap.String("key", key), zap.String("exp", expiration.String()))
	return result.Err()
}

func (cache Cache) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	result := cache.rDB.Set(ctx, key, value, expiration)
	logger := logs.GetLogger()
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
	logger := logs.GetLogger()
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
		return "", fmt.Errorf(string(request.KeyNotFoundError))
	case err != nil:
		return "", err
	case result == "":
		return "", fmt.Errorf(string(request.KeyNotFoundError))
	}
	logger := logs.GetLogger()
	logger.Debug("GETTING FROM REDIS", zap.String("key", key))
	return result, nil
}

func (cache Cache) GetJSON(ctx context.Context, key string, value interface{}) error {
	result := cache.rDB.Get(ctx, key)
	storedBytes, err := result.Bytes()
	if err != nil {
		return err
	}
	logger := logs.GetLogger()
	logger.Debug("GETTING FROM REDIS", zap.String("key", key))
	return json.Unmarshal(storedBytes, &value)
}

func (cache Cache) Delete(ctx context.Context, key string) error {
	return cache.rDB.Del(ctx, key).Err()
}

func (cache Cache) DeleteWithPattern(ctx context.Context, pattern string) error {
	logger := logs.GetLogger()
	logger.Debug("DELETING KEYS MATCHING PATTERN", zap.String("pattern", pattern))
	result, err := cache.rDB.Keys(ctx, pattern).Result()
	if err != nil {
		return err
	}
	return cache.rDB.Del(ctx, result...).Err()
}

// Hash Operations

// HGet gets a field value from a hash
func (cache Cache) HGet(ctx context.Context, hashName, key string) (string, error) {
	logger := logs.GetLogger()
	result, err := cache.rDB.HGet(ctx, hashName, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil // Return empty string like Node.js version
	}
	if err != nil {
		logger.Error("error in HGet", zap.String("hashName", hashName), zap.String("key", key), zap.Error(err))
		return "", err
	}
	return result, nil
}

// HSet sets a field value in a hash
func (cache Cache) HSet(ctx context.Context, hashName, key string, value interface{}) (int64, error) {
	logger := logs.GetLogger()
	result, err := cache.rDB.HSet(ctx, hashName, key, value).Result()
	if err != nil {
		logger.Error("Error while setting hashKey", zap.String("hashName", hashName), zap.String("key", key), zap.Any("value", value), zap.Error(err))
		return 0, err
	}
	return result, nil
}

// HDel deletes a field from a hash
func (cache Cache) HDel(ctx context.Context, hashName, key string) (int64, error) {
	logger := logs.GetLogger()
	result, err := cache.rDB.HDel(ctx, hashName, key).Result()
	if err != nil {
		logger.Error("Error while deleting hashKey", zap.String("hashName", hashName), zap.String("key", key), zap.Error(err))
		return 0, err
	}
	return result, nil
}

// HGetAll gets all fields and values from a hash
func (cache Cache) HGetAll(ctx context.Context, hashName string) (map[string]string, error) {
	logger := logs.GetLogger()
	result, err := cache.rDB.HGetAll(ctx, hashName).Result()
	if err != nil {
		logger.Error("error in HGetAll", zap.String("hashName", hashName), zap.Error(err))
		return map[string]string{}, err
	}
	return result, nil
}

// HKeys gets all field names from a hash
func (cache Cache) HKeys(ctx context.Context, hashName string) ([]string, error) {
	logger := logs.GetLogger()
	result, err := cache.rDB.HKeys(ctx, hashName).Result()
	if err != nil {
		logger.Error("error in getting hashKeys", zap.String("hashName", hashName), zap.Error(err))
		return []string{}, err
	}
	return result, nil
}

// HExists checks if a field exists in a hash
func (cache Cache) HExists(ctx context.Context, hashName, key string) (bool, error) {
	return cache.rDB.HExists(ctx, hashName, key).Result()
}

// HMGet gets multiple field values from a hash
func (cache Cache) HMGet(ctx context.Context, hashName string, keys ...string) ([]interface{}, error) {
	return cache.rDB.HMGet(ctx, hashName, keys...).Result()
}

// HMSet sets multiple field values in a hash
func (cache Cache) HMSet(ctx context.Context, hashName string, values map[string]interface{}) error {
	return cache.rDB.HMSet(ctx, hashName, values).Err()
}

// SetWholeHashMap sets multiple field values in a hash with expiration
func (cache Cache) SetWholeHashMap(ctx context.Context, hashName string, values map[string]interface{}, expiration time.Duration) error {
	pipe := cache.rDB.Pipeline()
	pipe.HMSet(ctx, hashName, values)
	if expiration > 0 {
		pipe.Expire(ctx, hashName, expiration)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// List Operations

// RPush pushes values to the right of a list
func (cache Cache) RPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	logger := logs.GetLogger()
	result, err := cache.rDB.RPush(ctx, key, values...).Result()
	if err != nil {
		logger.Error("error while pushing into list", zap.String("key", key), zap.Error(err))
		return 0, err
	}
	return result, nil
}

// LPop pops a value from the left of a list
func (cache Cache) LPop(ctx context.Context, key string) (string, error) {
	logger := logs.GetLogger()
	result, err := cache.rDB.LPop(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil // Return empty string when list is empty
	}
	if err != nil {
		logger.Error("error while lpop", zap.String("key", key), zap.Error(err))
		return "", err
	}
	return result, nil
}

// RPop pops a value from the right of a list
func (cache Cache) RPop(ctx context.Context, key string) (string, error) {
	logger := logs.GetLogger()
	result, err := cache.rDB.RPop(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil // Return empty string when list is empty
	}
	if err != nil {
		logger.Error("error while rpop", zap.String("key", key), zap.Error(err))
		return "", err
	}
	return result, nil
}

// LRange gets a range of values from a list
func (cache Cache) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	logger := logs.GetLogger()
	result, err := cache.rDB.LRange(ctx, key, start, stop).Result()
	if err != nil {
		logger.Error("error while fetching lrange", zap.String("key", key), zap.Error(err))
		return nil, err
	}
	return result, nil
}

// GetList gets all values from a list
func (cache Cache) GetList(ctx context.Context, key string) ([]string, error) {
	logger := logs.GetLogger()
	result, err := cache.rDB.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		logger.Error("error while getting list", zap.String("key", key), zap.Error(err))
		return []string{}, err
	}
	return result, nil
}

// MultiPush pushes multiple values to a list with expiration
func (cache Cache) MultiPush(ctx context.Context, key string, values []interface{}, expiration time.Duration) error {
	logger := logs.GetLogger()
	pipe := cache.rDB.Pipeline()
	for _, value := range values {
		pipe.RPush(ctx, key, value)
	}
	if expiration > 0 {
		pipe.Expire(ctx, key, expiration)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		logger.Error("error while multiPushing", zap.String("key", key), zap.Error(err))
	}
	return err
}

// ListRPOP pops multiple values from the right of a list
func (cache Cache) ListRPOP(ctx context.Context, key string, count int64) ([]string, error) {
	logger := logs.GetLogger()
	result, err := cache.rDB.RPopCount(ctx, key, int(count)).Result()
	if err != nil {
		logger.Error("error while rpop", zap.String("key", key), zap.Error(err))
		return nil, err
	}
	return result, nil
}

// Set Operations

// SAdd adds members to a set
func (cache Cache) SAdd(ctx context.Context, key string, members ...interface{}) (int64, error) {
	return cache.rDB.SAdd(ctx, key, members...).Result()
}

// SMembers gets all members of a set
func (cache Cache) SMembers(ctx context.Context, key string) ([]string, error) {
	logger := logs.GetLogger()
	result, err := cache.rDB.SMembers(ctx, key).Result()
	if err != nil {
		logger.Error("-- error inside getSetMembers", zap.String("key", key), zap.Error(err))
		return nil, err
	}
	return result, nil
}

// SCard gets the cardinality (number of members) of a set
func (cache Cache) SCard(ctx context.Context, key string) (int64, error) {
	logger := logs.GetLogger()
	result, err := cache.rDB.SCard(ctx, key).Result()
	if err != nil {
		logger.Error("-- error inside getSetCardinality", zap.String("key", key), zap.Error(err))
		return 0, err
	}
	return result, nil
}

// SPop removes and returns random members from a set
func (cache Cache) SPop(ctx context.Context, key string, count int64) ([]string, error) {
	logger := logs.GetLogger()
	result, err := cache.rDB.SPopN(ctx, key, count).Result()
	if err != nil {
		logger.Error("-- error inside popAndReturnFromSet", zap.String("key", key), zap.Error(err))
		return nil, err
	}
	return result, nil
}

// SIsMember checks if a value is a member of a set
func (cache Cache) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	logger := logs.GetLogger()
	result, err := cache.rDB.SIsMember(ctx, key, member).Result()
	if err != nil {
		logger.Error("-- error inside isSetMember", zap.String("key", key), zap.Error(err))
		return false, err
	}
	return result, nil
}

// SRem removes members from a set
func (cache Cache) SRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	logger := logs.GetLogger()
	result, err := cache.rDB.SRem(ctx, key, members...).Result()
	if err != nil {
		logger.Error("error while removing set member", zap.String("key", key), zap.Error(err))
		return 0, err
	}
	return result, nil
}

// UpsertToSet adds multiple members to a set
func (cache Cache) UpsertToSet(ctx context.Context, key string, members []interface{}) error {
	logger := logs.GetLogger()
	pipe := cache.rDB.Pipeline()
	for _, member := range members {
		pipe.SAdd(ctx, key, member)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		logger.Error("-- error inside upsertToSet", zap.String("key", key), zap.Error(err))
	}
	return err
}

// AddSetMembers adds members to a set with expiration
func (cache Cache) AddSetMembers(ctx context.Context, key string, members []interface{}, expiration time.Duration) (int64, error) {
	logger := logs.GetLogger()
	pipe := cache.rDB.Pipeline()
	addCmd := pipe.SAdd(ctx, key, members...)
	if expiration > 0 {
		pipe.Expire(ctx, key, expiration)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		logger.Error("-- error inside addSetMembers", zap.String("key", key), zap.Error(err))
		return 0, err
	}
	return addCmd.Val(), nil
}

// EmptyTheSet deletes a set
func (cache Cache) EmptyTheSet(ctx context.Context, key string) error {
	return cache.rDB.Del(ctx, key).Err()
}

// Locking and Advanced Operations

// Lock creates a lock with expiration (similar to setKey with NX option)
func (cache Cache) Lock(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	return cache.rDB.SetNX(ctx, key, "locked", expiration).Result()
}

// Unlock removes a lock
func (cache Cache) Unlock(ctx context.Context, key string) error {
	return cache.rDB.Del(ctx, key).Err()
}

// KeyExists checks if a key exists
func (cache Cache) KeyExists(ctx context.Context, key string) (bool, error) {
	logger := logs.GetLogger()
	count, err := cache.rDB.Exists(ctx, key).Result()
	if err != nil {
		logger.Error("error while checking for key", zap.String("key", key), zap.Error(err))
		return false, err
	}
	return count > 0, nil
}

// MultiSet sets multiple key-value pairs with expiration
func (cache Cache) MultiSet(ctx context.Context, pairs map[string]interface{}, expiration time.Duration) error {
	pipe := cache.rDB.Pipeline()
	for key, value := range pairs {
		pipe.Set(ctx, key, value, expiration)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// IncrBy increments a key by a specific amount with optional expiration
func (cache Cache) IncrBy(ctx context.Context, key string, amount int64, expiration time.Duration) (int64, error) {
	logger := logs.GetLogger()
	value, err := cache.rDB.IncrBy(ctx, key, amount).Result()
	if err != nil {
		logger.Error("error while incrementing in redis", zap.String("key", key), zap.Error(err))
		return 0, err
	}
	if expiration > 0 {
		cache.rDB.Expire(ctx, key, expiration)
	}
	return value, nil
}

// Decr decrements a key with optional expiration
func (cache Cache) Decr(ctx context.Context, key string, expiration time.Duration) (int64, error) {
	logger := logs.GetLogger()
	value, err := cache.rDB.Decr(ctx, key).Result()
	if err != nil {
		logger.Error("error while decrementing in redis", zap.String("key", key), zap.Error(err))
		return 0, err
	}
	if expiration > 0 {
		cache.rDB.Expire(ctx, key, expiration)
	}
	return value, nil
}

// IncrementWithExpire increments a key and sets expiration only if it's a new key
func (cache Cache) IncrementWithExpire(ctx context.Context, key string, expiration time.Duration) (int64, error) {
	logger := logs.GetLogger()
	value, err := cache.rDB.Incr(ctx, key).Result()
	if err != nil {
		logger.Error("error while incrementing in redis", zap.String("key", key), zap.Error(err))
		return 0, err
	}
	if value == 1 && expiration > 0 {
		cache.rDB.Expire(ctx, key, expiration)
	}
	return value, nil
}

// Pattern and Bulk Operations

// PatternReading scans and returns all keys matching a pattern
func (cache Cache) PatternReading(ctx context.Context, pattern string) ([]string, error) {
	var keys []string
	iter := cache.rDB.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	return keys, iter.Err()
}

// PatternDeletion deletes all keys matching a pattern with optional filter
func (cache Cache) PatternDeletion(ctx context.Context, pattern string, filter string) error {
	logger := logs.GetLogger()
	iter := cache.rDB.Scan(ctx, 0, pattern, 0).Iterator()
	pipe := cache.rDB.Pipeline()

	for iter.Next(ctx) {
		key := iter.Val()
		if filter == "" || contains(key, filter) {
			pipe.Del(ctx, key)
		}
	}

	if err := iter.Err(); err != nil {
		logger.Error("error occurred in patternDeletion", zap.String("pattern", pattern), zap.Error(err))
		return err
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		logger.Error("error occurred in patternDeletion", zap.String("pattern", pattern), zap.Error(err))
	}
	return err
}

// GetMultiKeys gets values for multiple keys
func (cache Cache) GetMultiKeys(ctx context.Context, keys []string) (map[string]string, error) {
	logger := logs.GetLogger()
	if len(keys) == 0 {
		return map[string]string{}, nil
	}

	values, err := cache.rDB.MGet(ctx, keys...).Result()
	if err != nil {
		logger.Error("error while get multi key values", zap.Strings("keys", keys), zap.Error(err))
		return map[string]string{}, err
	}

	result := make(map[string]string)
	for i, key := range keys {
		if values[i] != nil {
			result[key] = values[i].(string)
		}
	}
	return result, nil
}

// DelMultiKeys deletes multiple keys
func (cache Cache) DelMultiKeys(ctx context.Context, keys []string) (int64, error) {
	logger := logs.GetLogger()
	if len(keys) == 0 {
		return 0, nil
	}
	result, err := cache.rDB.Del(ctx, keys...).Result()
	if err != nil {
		logger.Error("error while deleting multi keys", zap.Strings("keys", keys), zap.Error(err))
		return 0, err
	}
	return result, nil
}

// GetMultipleKeyValues gets values for keys matching a pattern
func (cache Cache) GetMultipleKeyValues(ctx context.Context, keys []string) (map[string]string, error) {
	return cache.GetMultiKeys(ctx, keys)
}

// DeleteAllPossibleKeysByAString deletes all keys containing a specific string
func (cache Cache) DeleteAllPossibleKeysByAString(
	ctx context.Context,
	matchString string,
) (bool, error) {

	logger := logs.GetLogger()
	pattern := "*" + matchString + "*"

	var cursor uint64
	deletedAny := false

	for {
		keys, nextCursor, err := cache.rDB.Scan(ctx, cursor, pattern, 500).Result()
		if err != nil {
			logger.Error(
				"[DeleteAllPossibleKeysByAString] scan failed",
				zap.String("matchString", matchString),
				zap.Error(err),
			)
			return false, err
		}

		if len(keys) > 0 {
			_, err := cache.rDB.Del(ctx, keys...).Result()
			if err != nil {
				logger.Error(
					"[DeleteAllPossibleKeysByAString] delete failed",
					zap.String("matchString", matchString),
					zap.Strings("keys", keys),
					zap.Error(err),
				)
				return false, err
			}
			deletedAny = true
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return deletedAny, nil
}

// Helper function
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// Utility and Advanced Functions

// SetTTL sets expiration for an existing key
func (cache Cache) SetTTL(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	logger := logs.GetLogger()
	result, err := cache.rDB.Expire(ctx, key, expiration).Result()
	if err != nil {
		logger.Error("Error while setting expiration for key", zap.String("key", key), zap.Error(err))
		return false, err
	}
	return result, nil
}

// TTLMS gets the time to live in milliseconds
func (cache Cache) TTLMS(ctx context.Context, key string) (time.Duration, error) {
	logger := logs.GetLogger()
	result, err := cache.rDB.PTTL(ctx, key).Result()
	if err != nil {
		logger.Error("error while pttl fetch", zap.String("key", key), zap.Error(err))
		return 0, err
	}
	return result, nil
}

// SendCommand executes a custom Redis command
func (cache Cache) SendCommand(ctx context.Context, command string, args ...interface{}) (interface{}, error) {
	return cache.rDB.Do(ctx, append([]interface{}{command}, args...)...).Result()
}

// ExecuteMulti executes multiple Redis operations in a pipeline
func (cache Cache) ExecuteMulti(ctx context.Context, operations [][]interface{}) ([]interface{}, error) {
	pipe := cache.rDB.Pipeline()

	for _, operation := range operations {
		if len(operation) > 0 {
			pipe.Do(ctx, operation...)
		}
	}

	cmds, err := pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]interface{}, len(cmds))
	for i, cmd := range cmds {
		results[i] = cmd
	}

	return results, nil
}

// CheckIdsIfNotExists checks which IDs don't exist in a hash
func (cache Cache) CheckIdsIfNotExists(ctx context.Context, key string, ids []string) ([]string, error) {
	var missingIds []string

	for _, id := range ids {
		exists, err := cache.rDB.HExists(ctx, key, id).Result()
		if err != nil {
			return nil, err
		}
		if !exists {
			missingIds = append(missingIds, id)
		}
	}

	return missingIds, nil
}

// MultiMapSet sets multiple hash fields with JSON encoding and expiration
func (cache Cache) MultiMapSet(ctx context.Context, redisKey string, data map[string]interface{}, expiration time.Duration) error {
	pipe := cache.rDB.Pipeline()

	for id, value := range data {
		jsonValue, err := json.Marshal(value)
		if err != nil {
			return err
		}
		pipe.HSet(ctx, redisKey, id, string(jsonValue))
	}

	if expiration > 0 {
		pipe.Expire(ctx, redisKey, expiration)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// SetWithNX sets a key only if it doesn't exist (NX option)
func (cache Cache) SetWithNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	return cache.rDB.SetNX(ctx, key, value, expiration).Result()
}
