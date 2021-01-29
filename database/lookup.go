package database

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// LookupType represents the Code's Domain
type LookupType struct {
	ID     primitive.ObjectID `json:"-" bson:"_id"` // id kann im json/response weggelassen werden
	Name   string             `json:"lookupType" bson:"codeType"`
	Values []LookupValue      `json:"values" bson:"values"`
}

// LookupValue represents an Item of the Code's Domain
type LookupValue struct {
	LookupValue int32  `json:"lookupValue" bson:"codeValue"`
	Disabled    bool   `json:"disabled" bson:"disabled"`
	Default     bool   `json:"default" bson:"default"`
	Indicator   string `json:"indicator" bson:"indicator"` // ToDo: "none" weglassen mit omitempty
	TextEN      string `json:"textEN" bson:"codeTextEN"`
	TextDE      string `json:"textDE" bson:"codeTextDE"`
}

// GetLookupText returns Text to Code (ToDO: Language)
func GetLookupText(lookupType string, lookupValue int32) string {
	// https://stackoverflow.com/questions/38654383/how-to-search-for-an-element-in-a-golang-slice
	str := ""

	for t := range lookups {
		if lookups[t].Name == lookupType {
			for v := range lookups[t].Values {
				if lookups[t].Values[v].LookupValue == lookupValue {
					str = lookups[t].Values[v].TextEN
					return str
				}
			}
		}
	}

	return str
}

// GetLookupValue attempts to find the code value of an english string
func GetLookupValue(lookupType string, lookupText string) (int32, error) {

	var val int32 = -1
	str := strings.ToLower(lookupText)

	for t := range lookups {
		if lookups[t].Name == lookupType {
			for v := range lookups[t].Values {
				if strings.ToLower(lookups[t].Values[v].TextEN) == str {
					val = lookups[t].Values[v].LookupValue
					return val, nil
				}
			}
		}
	}

	return val, errors.New("not found")
}

// internal loader of the code-map, used only by "OpenConnection"
// (handlers retrieves the data via the the singleton)
func getLookupMap() ([]LookupType, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	// get a collection to interact with (would be created as needed)
	collection := client.Database(os.Getenv("DB_NAME")).Collection("system")

	// Dokument entspricht der Beschreibung im Manual
	// https://docs.mongodb.com/manual/reference/operator/query/exists/
	filter := bson.D{{Key: "codeType", Value: bson.D{{Key: "$exists", Value: "true"}}}}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}

	var lookupTypes []LookupType
	if err = cursor.All(ctx, &lookupTypes); err != nil {
		return nil, err
	}

	return lookupTypes, nil
}
