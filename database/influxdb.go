package database

import (
	"context"
	"os"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go"
	"github.com/influxdata/influxdb-client-go/api"
)

type InfluxAPI struct {
	WriteAPI  api.WriteAPIBlocking // ToDo: auf non-blocking umstellen
	QueryAPI  api.QueryAPI
	DeleteAPI api.DeleteAPI
}

// client remains private
var influxClient influxdb2.Client

// OpenInfluxConnection pools the connection to the store
func OpenInfluxConnection() error {
	url := os.Getenv("ANALYTICS_URL")
	token := os.Getenv("ANALYTICS_TOKEN")

	influxClient = influxdb2.NewClient(url, token)
	influxClient.Options().SetPrecision(time.Second)

	// check if alright so far
	// ToDO: Read Status in eror umwandeln
	var ctx = context.Background()
	_, err := influxClient.Ready(ctx)
	if err != nil {
		return err
	}

	return nil
}

// GetInfluxConnection returns a reference to the shared connection
func GetInfluxConnection() *influxdb2.Client {
	return &influxClient
}

// CloseInfluxConnection closes the connection to the store
func CloseInfluxConnection() {
	influxClient.Close()
}
