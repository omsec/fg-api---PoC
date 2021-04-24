package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// shared connection (private to members of this package)
var client *mongo.Client

// since there are no joins in MongoDB, look-ups to texts that describe values (selection options)
// are integrated into the client's "core"
var lookups []LookupType

// OpenConnection to the database
func OpenConnection() error {
	var err error

	conStr := fmt.Sprintf("mongodb://%s:%s@%s:%s",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASS"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"))

	client, err = mongo.NewClient(options.Client().ApplyURI(conStr))
	if err != nil {
		return err
	}

	// every caller will create its own context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen
	err = client.Connect(ctx)
	if err != nil {
		return err
	}

	// make sure a connection has actually been made
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		return err
	}

	// load look-up map (singleton)
	if lookups == nil {
		lookups, err = getLookupMap()
		if err != nil {
			return err
		}
	}

	return nil
}

// CloseConnection closes the connection to the DB (when client is shut-down)
func CloseConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()                // nach 10 Sekunden abbrechen
	return client.Disconnect(ctx) // möglicher Fehler weitergeben
}

// GetConnection returns a reference to the shared connection
func GetConnection() *mongo.Client {
	return client
}

// GetLookups returns a reference to the map of code definitions
func GetLookups() ([]LookupType, error) {
	if lookups == nil {
		return nil, errors.New("look-up not available")
	}

	return lookups, nil
}
