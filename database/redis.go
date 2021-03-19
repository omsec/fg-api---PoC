package database

import (
	"context"
	"os"
	"strconv"

	// manuell eingetragen (unterhalt der version ohne /v8)
	"github.com/go-redis/redis/v8"
)

var redisClient *redis.Client

// OpenRedisConnection pools the connection to the store
func OpenRedisConnection() error {
	var err error

	var dsn string
	dsn = os.Getenv("CACHE_HOST") + ":" + os.Getenv("CACHE_PORT")

	dbID, err := strconv.Atoi(os.Getenv("ANALYTICS_DB"))
	if err != nil {
		return err
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr:     dsn,
		Password: os.Getenv("CACHE_PASS"),
		DB:       dbID,
	})

	// ping aufruf vermutlich, dass der compiler ruhig ist
	var ctx = context.Background()
	_, err = redisClient.Ping(ctx).Result()
	if err != nil {
		return err
	}

	return nil
}

// GetConnection returns a reference to the shared connection
func GetRedisConnection() *redis.Client {
	return redisClient
}

// CloseRedisConnection closes the connection to the store
func CloseRedisConnection() error {
	return redisClient.Close()
}
