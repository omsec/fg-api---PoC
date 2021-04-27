package analytics

import (
	"context"
	"fmt"
	"forza-garage/client"
	"forza-garage/database"
	"forza-garage/helpers"
	"forza-garage/lookups"
	"forza-garage/models"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Tracker struct {
	influxClient influxdb2.Client
	VisitorAPI   database.InfluxAPI
	SearchAPI    database.InfluxAPI
	collections  map[string]*mongo.Collection
	GetUserName  func(ID string) (string, error)
	// GetUserNameOID func(userID primitive.ObjectID) (string, error) // war für alte Lösung
	Requests *client.Registry
}

type Visit struct {
	VisitTS  time.Time `json:"visitTS"`
	ObjectID string
	UserID   string `json:"userID"`
	UserName string `json:"userName"`
}

// SearchParams is used to pass generic search parameters to the logger
type SearchParams struct {
	Domain      string
	ProfileIDs  []string // https://www.socketloop.com/tutorials/golang-how-to-get-struct-field-and-value-by-name
	GameCode    int32
	SeriesCodes []int32
	SearchTerm  string
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

// SetConnections initializes the instance
func (t *Tracker) SetConnections(influxClient *influxdb2.Client, mongoCollections map[string]*mongo.Collection) {
	t.influxClient = *influxClient
	t.collections = mongoCollections
}

// SaveVisitor stores event data in the analytics cache
func (t *Tracker) SaveVisitor(domain string, profileID string, userID string) {

	if os.Getenv("USE_ANALYTICS") != "YES" {
		return
	}

	// include object type (domain) in key name,
	// so this information can be "wrapped" in aggegration queries (eq "select profileID, count")

	// the risk of high series cardinalty is accepted, since profiles is what we're interessted in
	// https://docs.influxdata.com/influxdb/v2.0/write-data/best-practices/resolve-high-cardinality/

	p := influxdb2.NewPoint(
		"visit",
		map[string]string{"profileId": domain + "_" + profileID},
		map[string]interface{}{"userId": userID},
		time.Now())

	// ToDo: log Error
	t.VisitorAPI.WriteAPI.WritePoint(p)

}

// SaveSearch stores event data in the analytics cache
// Logger Functions are typed due to different fields/logic of the domains
func (t *Tracker) SaveSearchCourse(search *models.CourseSearchParams, results []models.CourseListItem) {

	// zum testen auskommentieren
	if os.Getenv("USE_ANALYTICS") != "YES" {
		return
	}

	// do not log any empty search
	if search.SearchTerm == "" {
		return
	}

	// do not log searches for std-routes (usually look-ups)
	if search.SearchMode == models.CourseSearchModeStandard {
		return
	}

	/*
		// do not log "homepage" (no filters)
		if len(search.SeriesCodes) == 3 && search.SearchTerm == "" {
			return
		}
	*/

	// convert seriesCodes into indicators
	road, dirt, cross := t.seriesIndicators(search.SeriesCodes)

	// searched series will be stored bitwise to save storage, according to this:
	// https://www.mssqltips.com/sqlservertip/1218/sql-server-bitwise-operators-to-store-multiple-values-in-one-column/
	series := byte(math.Pow(2*road, 1) + math.Pow(2*dirt, 2) + math.Pow(2*cross, 3))

	ts := time.Now()

	for _, v := range results {
		fields := map[string]interface{}{
			"domain": "course",
			"game":   search.GameCode,
			"series": series,
			"term":   search.SearchTerm}

		p := influxdb2.NewPoint(
			"search", // measurement
			map[string]string{"courseId": v.ID.Hex()}, // tag
			fields,
			ts)

		// ToDo: log Error
		t.SearchAPI.WriteAPI.WritePoint(p)
	}

	// flush called implicity (nicht perfekt)
}

// --> alte Lösung - nicht mehr gebraucht
/*
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
	t.SearchAPI.WriteAPI.WritePoint(p)

}
*/

// GetVisits counts the number of visits of a profile
// the value is "live" - meaning it's read from the analytics cache (influxDB)
// which is set to a maximum period (TTL) of 30 days
// creators and admins may receive the total counts which is added by the MongoDB information (different, protected endpoint)
func (t *Tracker) GetVisits(domain string, profileID string, startDT time.Time) (int64, error) {

	// zum testen auskommentieren
	if os.Getenv("USE_ANALYTICS") != "YES" {
		return -1, nil
	}

	flux := `from(bucket: "%s")
		|> range(start: %s)
		|> filter(fn: (r) => r["_measurement"] == "visit" and r["profileId"] == "%s")
		|> count()
		|> yield(name: "count")`

	id := domain + "_" + profileID
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

	// alte Lösung
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
func (t *Tracker) ListVisitors(profileID string, startDT time.Time, userID string) ([]Visit, error) {

	// zum testen auskommentieren
	if os.Getenv("USE_ANALYTICS") != "YES" {
		return nil, nil
	}

	// 10 letzte Besucher (nur letzter Besuch pro Benutzer)
	flux := `import "strings"
		from(bucket: "%s")
		|> range(start: %s)
		|> filter(fn: (r) => r["_measurement"] == "visit" and strings.containsStr(substr: "%s", v: r.profileId))
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

	flux = fmt.Sprintf(
		flux,
		os.Getenv("ANALYTICS_VISITORS_BUCKET"),
		startDT.Format(time.RFC3339), // 2021-04-01T00:00:00Z
		profileID)

	result, err := t.SearchAPI.QueryAPI.Query(context.Background(), flux)
	if err != nil {
		return nil, helpers.WrapError(err, helpers.FuncName())
	}

	var visit Visit
	var visits []Visit
	for result.Next() {
		visit.VisitTS = result.Record().Time()
		visit.ObjectID = profileID
		if result.Record().Value() == nil {
			visit.UserID = ""
			visit.UserName = ""
		} else {
			visit.UserID = result.Record().Value().(string)
			visit.UserName, _ = t.GetUserName(visit.UserID)
		}

		visits = append(visits, visit)
	}

	// das flux query ist zumindest im GUI richtig sortiert, das slice kommt aber anders daher
	// https://he-the-great.livejournal.com/49072.html
	sort.Slice(visits, func(i, j int) bool {
		return visits[j].VisitTS.Before(visits[i].VisitTS)
	})

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

	ctx := context.Background()
	// https://golangbyexample.com/create-new-time-instance-go/
	//start := time.Parse() "2021-01-01T00:00:00Z"
	start := time.Date(2021, 1, 1, 0, 0, 0, 0, time.Now().UTC().Location()) // just start somewhere as the minimum date
	stop := time.Now().AddDate(0, -1, 0)                                    // subtract 1 month to move everything older than one month
	//stop := time.Now().AddDate(0, 0, -1)
	//stop := time.Now()

	// 1. get counts from influxDB

	/*
		from(bucket: "fg-visits")
				|> range(start: 2021-01-01T00:00:00Z, stop: -30d) // just start somewhere as the minimum date
				|> filter(fn: (r) => r["_measurement"] == "visit" and r["_field"] == "domain")
				|> count()
				|> yield(name: "count")
	*/

	flux := `from(bucket: "%s")
	|> range(start: %s, stop: %s) // use pre-calculated stop because delete-api needs time
	|> filter(fn: (r) => r["_measurement"] == "visit")
	|> count()
	|> yield(name: "count")`

	flux = fmt.Sprintf(
		flux,
		os.Getenv("ANALYTICS_VISITORS_BUCKET"),
		start.Format(time.RFC3339),
		stop.Format(time.RFC3339))

	result, err := t.SearchAPI.QueryAPI.Query(ctx, flux)
	if err != nil {
		// ToDO: Log Error helpers.WrapError(err, helpers.FuncName())
		fmt.Println(helpers.WrapError(err, helpers.FuncName()))
		return
	}

	// 2. save counts to MongoDB (bulk)
	// ToDo: receive map of collections and handle them

	// https://docs.mongodb.com/manual/reference/method/db.collection.bulkWrite/
	// https://pkg.go.dev/go.mongodb.org/mongo-driver/mongo#Collection.BulkWrite

	// https://stackoverflow.com/questions/58538657/golang-mongodb-bulkwrite-to-update-slice-of-documents

	// create a write model for each collection
	// some domains (object types) are stored in the same collection, eg. course & championship
	opModels := make(map[string][]mongo.WriteModel)
	//var operation bson.D

	/*
		// used for debugging
		type testProfile struct {
			id  string
			cnt int64
		}
		var testProfiles []testProfile
	*/

	var strs []string // used to "extract" object type from key
	for result.Next() {
		// create a document and a write model for each record

		strs = strings.Split(result.Record().ValueByKey("profileId").(string), "_")

		operation := bson.D{
			{Key: "$inc", Value: bson.D{
				{Key: "metaInfo.visits", Value: result.Record().Value()}, // value of the projection function (count)
			}},
		}

		opModel := mongo.NewUpdateOneModel()
		opModel.SetFilter(bson.D{{Key: "_id", Value: helpers.ObjectID(strs[1])}}).SetUpdate(operation)

		// fmt.Printf("%v: %v\n", strs[0], strs[1])
		// die objekt-typen (domains) aus der influxDB auf collections der mongoDB mappen
		switch strs[0] {
		case "user":
			opModels["users"] = append(opModels["users"], opModel)
		case "course", "championship":
			opModels["racing"] = append(opModels["racing"], opModel)
		default:
			// ToDo: Log
			fmt.Println("ERROR: repl not correctly implemented")
		}

		/*
			// used for debugging
			// fmt.Printf("profile: %v, %v: %v\n", result.Record().Field(), result.Record().ValueByKey("profileId").(string), result.Record().Value())
			tp := testProfile{
				id:  result.Record().ValueByKey("profileId").(string),
				cnt: result.Record().Value().(int64)}
			testProfiles = append(testProfiles, tp)
		*/
	}

	// fmt.Println(testProfiles)

	// len returns int, mongoDB's matchCount int64
	// to avoid all the conversions, two variables
	// are used for actually the same thing
	var i int = 0
	for _, v := range opModels {
		// fmt.Printf("tst: %v", len(v))
		i += len(v)
	}

	// abort if no data to process
	if i == 0 {
		// TODO: Log
		fmt.Printf("%v: %v profile's visit(s) replicated.\n", time.Now().Format(time.RFC3339), 0)
		return
	}

	opts := options.BulkWrite().SetOrdered(false)

	var cnt int64 = 0 // total replicated profile's visits

	// process each collection's write models (= update operations)
	for k, v := range opModels {
		if v != nil {
			res, err := t.collections[k].BulkWrite(ctx, v, opts) // context noch unklar, background ist nicht cancellable
			if err != nil {
				// ToDO: Log Error helpers.WrapError(err, helpers.FuncName())
				fmt.Println(helpers.WrapError(err, helpers.FuncName()))
			}
			cnt = res.MatchedCount
		}
	}

	// ToDo: could be logged
	fmt.Printf("%v: %v profile's visit(s) replicated.\n", time.Now().Format(time.RFC3339), cnt)

	// 3. delete transfered data from influxDB
	/*
		err = t.VisitorAPI.DeleteAPI.DeleteWithName(ctx, os.Getenv("ANALYTICS_ORG"), os.Getenv("ANALYTICS_VISITORS_BUCKET"), start, stop, "")
		if err != nil {
			// ToDo: Log "real" (severe) error
			fmt.Println("ERROR: could not delete data in influxDB that was already written to MongoDB => duplicated/high values")
			return
		}
	*/
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
