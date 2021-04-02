package analytics

import (
	"context"
	"fmt"
	"forza-garage/database"
	"forza-garage/helpers"
	"forza-garage/lookups"
	"math"
	"os"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go"
	"go.mongodb.org/mongo-driver/mongo"
)

type Tracker struct {
	influxClient influxdb2.Client
	VisitorAPI   database.InfluxAPI
	SearchAPI    database.InfluxAPI
	collection   *mongo.Collection
	GetUserName  func(ID string) (string, error)
	// GetUserNameOID func(userID primitive.ObjectID) (string, error) // war für alte Lösung
}

type Visit struct {
	VisitTS  time.Time `json:"visitTS"`
	ObjectID string
	UserID   string `json:"userID"`
	UserName string `json:"userName"`
}

/*
// alte Lösung
// VisitCache is the list item in the cache (redis) -ToDO
type VisitCache struct {
	VisitTS time.Time `json:"visitTS"`
	UserID  string    `json:"userID"`
}

// Visit is the final data structure to be stored in the database (MongoDB)
// and sent to the client when requested; hence "the model"
type Visit struct {
	ID         primitive.ObjectID `json:"-" bson:"_id"`
	VisitTS    time.Time          `json:"visitTS" bson:"visitTS"`
	ObjectType string             `json:"-" bson:"objectType"`
	ObjectID   primitive.ObjectID `json:"-" bson:"objectID"`
	UserID     primitive.ObjectID `json:"userID" bson:"userID,omitempty"`
	UserName   string             `json:"userName" bson:"userName,omitempty"`
}
*/

// ToDo
func (t *Tracker) SetConnections(influxClient *influxdb2.Client, mongoCollection *mongo.Collection) {
	t.influxClient = *influxClient
	t.collection = mongoCollection
}

// SaveVisitor stores event data in the analytics cache
func (t *Tracker) SaveVisitor(objectType string, objectID string, userID string) {

	if os.Getenv("USE_ANALYTICS") != "YES" {
		return
	}

	p := influxdb2.NewPoint(
		"visit",
		map[string]string{"profileId": objectType + "_" + objectID},
		map[string]interface{}{"userId": userID},
		time.Now())

	// ToDo: log Error
	t.VisitorAPI.WriteAPI.WritePoint(context.Background(), p)

}

// SaveSearch stores event data in the analytics cache
// series is a masked integer
func (t *Tracker) SaveSearch(domain string, gameCode int32, seriesCodes []int32, searchTerm string, userID string) {

	// zum testen auskommentieren
	if os.Getenv("USE_ANALYTICS") != "YES" {
		return
	}

	// do not log "homepage" (no filters)
	if len(seriesCodes) == 3 && searchTerm == "" {
		return
	}

	// convert seriesCodes into indicators
	road, dirt, cross := t.seriesIndicators(seriesCodes)
	// indicators

	// searched series will be stored bitwise to save storage, according to this:
	// https://www.mssqltips.com/sqlservertip/1218/sql-server-bitwise-operators-to-store-multiple-values-in-one-column/
	series := byte(math.Pow(2*road, 1) + math.Pow(2*dirt, 2) + math.Pow(2*cross, 3))

	fields := map[string]interface{}{
		"game":   gameCode,
		"series": series,
		"term":   searchTerm}

	p := influxdb2.NewPoint(
		"search",
		map[string]string{"domain": domain}, // course, championship ...
		fields,
		time.Now())

	// ToDo: log Error
	t.SearchAPI.WriteAPI.WritePoint(context.Background(), p)

}

// GetVisits reads peristed visitor data from the database
func (t *Tracker) GetVisits(objectType string, objectID string, startDT time.Time) (int64, error) {

	// zum testen auskommentieren
	if os.Getenv("USE_ANALYTICS") != "YES" {
		return -1, nil
	}

	// evtl. nur Visits today - total count für owner

	flux := `from(bucket: "%s")
		|> range(start: %s)
		|> filter(fn: (r) => r["_measurement"] == "visit" and r["profileId"] == "%s")
		|> count()
		|> yield(name: "count")`

	id := objectType + "_" + objectID
	flux = fmt.Sprintf(
		flux,
		os.Getenv("ANALYTICS_VISITORS_BUCKET"),
		startDT.Format(time.RFC3339),
		id)

	result, err := t.SearchAPI.QueryAPI.Query(context.Background(), flux)
	if err != nil {
		return 0, helpers.WrapError(err, helpers.FuncName())
	}

	// nur 1 record
	var res interface{}
	for result.Next() {
		res = result.Record().Value()
	}

	var cnt int64 = 0
	if res != nil {
		cnt = res.(int64)
	}

	return cnt, nil

	// Gesamtlösung noch etwas unklar :-)
	// Eventuell mit Replikation in die MongoDB zur Langzeitspeicherung (aktuell TTL 30d auf der Collection!)
	// momemtan Live-Auswertung aus der InfluxDB (TTL setzen)
	/*
		// 1. count documents in mongoDB
		oid, err := primitive.ObjectIDFromHex(objectID)
		if err != nil {
			return 0, helpers.WrapError(err, helpers.FuncName())
		}

		filter := bson.D{
			{Key: "visitTS", Value: bson.D{
				{Key: "$gte", Value: startDT},
			}},
			{Key: "objectID", Value: oid},
		}

		opts := options.Count().SetMaxTime(2 * time.Second)
		// ToDo: Hint & Index erstellen (sort desc)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel() // nach 10 Sekunden abbrechen

		cnt, err := t.collection.CountDocuments(ctx, filter, opts)
		if err != nil {
			return 0, helpers.WrapError(err, helpers.FuncName())
		}

		// 2. also check for data in the cache that's not yet replicated (optional, ToDO: Influx)
		return cnt, nil
	*/
}

// ListVisitors
// noch überlegen, wie anwenden, evtl. eine "stats"-seite für den user (link via usr-prile)
// die würde dann alles zeigen, was von diesem autor erstellt wurde (params somit anpassen)
// ==> vermutlich hier mal eine section, die nur bei Creators und Admins eingeblendet wird.
func (t *Tracker) ListVisitors(objectType string, objectID string, startDT time.Time, userID string) ([]Visit, error) {

	// zum testen auskommentieren
	if os.Getenv("USE_ANALYTICS") != "YES" {
		return nil, nil
	}

	// 10 letzte Besucher (nur letzter Besuch pro Benutzer)
	// die records werden trotzdem nach _value (userId) sortiert - das müsste wohl im client korrigiert werden
	flux := `from(bucket: "%s")
		|> range(start: %s)
		|> filter(fn: (r) => r["_measurement"] == "visit" and r["profileId"] == "%s")
		|> group(columns: ["_value"], mode:"by")
		|> max(column: "_time")
		|> sort(columns: ["_time"], desc: true)
		|> limit(n:10, offset: 0)`

	// 10 letzte Besuche (gesamte Liste, gleicher Benutzer mehrfach möglich)
	/*
		flux := `from(bucket: "%s")
		   			|> range(start: %s)
		   			|> filter(fn: (r) => r["_measurement"] == "visit" and r["profileId"] == "%s")
		     		|> sort(columns: ["_time"], desc: true)
		     		|> limit(n:10, offset: 0)`
	*/

	id := objectType + "_" + objectID
	flux = fmt.Sprintf(
		flux,
		os.Getenv("ANALYTICS_VISITORS_BUCKET"),
		startDT.Format(time.RFC3339), // 2021-04-01T00:00:00Z
		id)

	result, err := t.SearchAPI.QueryAPI.Query(context.Background(), flux)
	if err != nil {
		return nil, helpers.WrapError(err, helpers.FuncName())
	}

	var visit Visit
	var visits []Visit
	for result.Next() {
		visit.VisitTS = result.Record().Time()
		visit.ObjectID = objectID
		if result.Record().Value() == nil {
			visit.UserID = ""
			visit.UserName = ""
		} else {
			visit.UserID = result.Record().Value().(string)
			visit.UserName, _ = t.GetUserName(visit.UserID)
		}

		visits = append(visits, visit)
	}

	return visits, nil

	/*
		// alte Lösung
		// 1. look for "hot" data in cache (which is not yet replicated) - optional
		// currently NOT intended

		oid, err := primitive.ObjectIDFromHex(objectID)
		if err != nil {
			return nil, helpers.WrapError(err, helpers.FuncName())
		}

		// https://www.mongodb.com/blog/post/quick-start-golang--mongodb--data-aggregation-pipeline
		// https://docs.mongodb.com/manual/core/aggregation-pipeline/index.html
		// https://docs.mongodb.com/manual/reference/operator/aggregation/max/

		// build list select max(visitTS), userName where oid=X and visitTS>=Y group by userName order by maxTS desc, limit 10

		// 2. get data from MongoDB
		// https://docs.mongodb.com/manual/reference/operator/aggregation-pipeline/#aggregation-pipeline-operator-reference
		//matchStage := bson.D{{Key: "$match", Value: bson.D{{Key: "objectID", Value: oid}}}}
		matchStage := bson.D{
			{Key: "$match", Value: bson.D{
				{Key: "$and", Value: bson.A{
					bson.D{{Key: "objectID", Value: oid}},
					bson.D{{Key: "visitTS", Value: bson.D{
						{Key: "$gte", Value: startDT},
					}}},
				}},
			}},
		}

		groupStage := bson.D{
			{Key: "$group", Value: bson.D{
				{Key: "_id", Value: "$userID"},
				{Key: "lastVisit", Value: bson.D{
					{Key: "$max", Value: "$visitTS"},
				},
				}},
			}}

		// Resulat am Schluss sortieren und limitieren
		sortStage := bson.D{{Key: "$sort", Value: bson.D{{Key: "lastVisit", Value: -1}}}} // desc
		limitStage := bson.D{{Key: "$limit", Value: 5}}

		// ToDo: Hint (to use index)
		// https://www.unitconverters.net/time/second-to-nanosecond.htm
		opts := options.Aggregate().SetMaxTime(5000000000) // 5 secs

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel() // nach 10 Sekunden abbrechen

		cursor, err := t.collection.Aggregate(ctx, mongo.Pipeline{
			matchStage,
			groupStage,
			sortStage,
			limitStage}, opts)
		if err != nil {
			return nil, helpers.WrapError(err, helpers.FuncName())
		}

		var visitsDB []bson.M
		if err = cursor.All(ctx, &visitsDB); err != nil {
			return nil, helpers.WrapError(err, helpers.FuncName())
		}

		// check for empty result set (no error raised by find)
		if visitsDB == nil {
			return nil, apperror.ErrNoData
		}

		// MDB-TS->GoTime
		// https://stackoverflow.com/questions/64418512/how-to-convert-mongodb-go-drivers-primitive-timestamp-type-back-to-golang-time

		// ..to MongoDB-Timestamp:
		// "lastUpdate": primitive.Timestamp{T:uint32(time.Now().Unix())}

		var visits []Visit
		var visit Visit
		for _, v := range visitsDB {
			mts := v["lastVisit"].(primitive.DateTime)
			visit.VisitTS = mts.Time()
			if v["_id"] == nil {
				visit.UserID = primitive.NilObjectID
				visit.UserName = ""
			} else {
				visit.UserID = v["_id"].(primitive.ObjectID)
				visit.UserName, _ = t.GetUserNameOID(v["_id"].(primitive.ObjectID))
				//visit.UserName = v["_id"].(string)
			}

			visits = append(visits, visit)
			//fmt.Println(v["_id"])
		}

		return visits, nil
	*/
}

// Replicate moves the visits from the cache (InfluxDB) into the database (Mongo)
func (t *Tracker) Replicate() {
	fmt.Println("Replicating...")

	// ausführen jede Stunde
	// älter 30 tage
	// summiere in MongoDB (oid, count)

	// 3. save visits into MongoDB (wieder einbauen)

}

func (t *Tracker) seriesIndicators(seriesCodes []int32) (road float64, dirt float64, cross float64) {
	for _, v := range seriesCodes {
		if v == lookups.SeriesRoad {
			road = 1
		}
		if v == lookups.SeriesDirt {
			dirt = 1
		}
		if v == lookups.SeriesCross {
			cross = 1
		}
	}
	return road, dirt, cross
}
