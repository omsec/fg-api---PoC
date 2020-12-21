package authentication

import (
	"context"
	"os"
	"strconv"

	// manuell eingetragen (unterhalt der version ohne /v8)
	"github.com/go-redis/redis/v8"
)

var client *redis.Client

// OpenConnection pools the connection to the store
func OpenConnection() error {
	var err error

	var dsn string
	dsn = os.Getenv("JWT_HOST") + ":" + os.Getenv("JWT_PORT")

	dbID, err := strconv.Atoi(os.Getenv("JWT_DB"))
	if err != nil {
		return err
	}

	client = redis.NewClient(&redis.Options{
		Addr:     dsn,
		Password: os.Getenv("JWT_PASS"),
		DB:       dbID,
	})

	// ping aufruf vermutlich, dass der compiler ruhig ist
	var ctx = context.Background()
	_, err = client.Ping(ctx).Result()
	if err != nil {
		return err
	}

	return nil
}

// CloseConnection closes the connection to the store
func CloseConnection() error {
	return client.Close()
}
