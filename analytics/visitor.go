package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"forza-garage/helpers"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/twinj/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Tracker struct {
	redisClient    *redis.Client
	collection     *mongo.Collection
	GetUserName    func(ID string) (string, error)
	GetUserNameOID func(userID primitive.ObjectID) (string, error)
}

// VisitCache is the list item in the cache (redis)
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

func (t *Tracker) SetConnections(redisClient *redis.Client, mongoCollection *mongo.Collection) {
	t.redisClient = redisClient
	t.collection = mongoCollection
}

// SaveVisitor stores event data in the cache
func (t *Tracker) SaveVisitor(objectType string, objectID string, userID string) {

	if os.Getenv("USE_ANALYTICS") != "YES" {
		return
	}

	// fmt.Println("Tracking...")

	var ctx = context.Background()

	key := objectType + "_" + objectID + "_" + uuid.NewV4().String()

	//fmt.Println(key)

	profileVisit := VisitCache{
		VisitTS: time.Now(),
		UserID:  userID,
	}

	// erzeugt []byte
	b, err := json.Marshal(profileVisit)
	if err != nil {
		fmt.Println(err) // ToDO: Loggen, abbruch im Fehlerfall durch return
		return
	}

	err = t.redisClient.Set(ctx, key, b, 0).Err()
	if err != nil {
		fmt.Println(err) // ToDO: Loggen, abbruch im Fehlerfall durch return
	}

}

// GetVisits
// ToDO: name t "this" in general
func (t *Tracker) GetVisits(objectID string, startDT time.Time) (int64, error) {

	// zum testen auskommentieren
	if os.Getenv("USE_ANALYTICS") != "YES" {
		return -1, nil
	}

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

	// 2. also check for data in the cache that's not yet replicated (optional)
	allKeys, err := t.getKeys("*" + objectID + "*")
	if err != nil {
		fmt.Println(err)
	}

	//fmt.Println(allKeys)
	if allKeys != nil {
		cnt += int64(len(allKeys))
	}

	return cnt, nil
}

// ListVisitors
// noch überlegen, wie anwenden, evtl. eine "stats"-seite für den user (link via usr-prile)
// die würde dann alles zeigen, was von diesem autor erstellt wurde (params somit anpassen)
// ==> vermutlich hier mal eine section, die nur bei Creators und Admins eingeblendet wird.
func (t *Tracker) ListVisitors(objectID string, startDT time.Time, userID string) ([]Visit, error) {

	// zum testen auskommentieren
	if os.Getenv("USE_ANALYTICS") != "YES" {
		return nil, nil
	}

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
}

// Replicate moves the visits from the cache (Redis) into the database (Mongo)
func (t *Tracker) Replicate() {
	fmt.Println("Replicating...")

	var ctx = context.Background()
	var err error

	var allKeys []string

	var visits []Visit

	// 1. get all keys in DB
	allKeys, err = t.getKeys("*")
	if err != nil {
		return // abort in case of an error
	}

	// abort if no data found
	if allKeys == nil {
		return
	}

	// 2. extract values and build target structure
	vc := VisitCache{}
	vd := Visit{}      // database target
	var parts []string // used to split key name
	var userName = ""
	for _, key := range allKeys {
		val, err := t.redisClient.Get(ctx, key).Result()
		if err != nil {
			fmt.Println(err) // ToDO: Log
			return           // abort entire function in case of an error Altenrative: noch eine ArrayListe bauen mit "korrekten"
		}

		json.Unmarshal([]byte(val), &vc)

		vd.ID = primitive.NewObjectID()
		vd.VisitTS = vc.VisitTS

		parts = strings.Split(key, "_")
		vd.ObjectType = parts[0]
		vd.ObjectID, _ = primitive.ObjectIDFromHex(parts[1])

		if vc.UserID != "" {
			vd.UserID, err = primitive.ObjectIDFromHex(vc.UserID)
			// any error treated as anonymous user
			if err != nil {
				vd.UserID = primitive.NilObjectID
				vd.UserName = ""
			} else {
				// ok
				userName, err = t.GetUserName(vc.UserID)
				if err == nil {
					vd.UserName = userName
				}
			}
		} else {
			// anonymous user (not logged-in)
			vd.UserID = primitive.NilObjectID
			vd.UserName = ""
		}
		visits = append(visits, vd)
	}

	// abort if no data to process
	if visits == nil {
		return
	}

	// 3. save visits into MongoDB (private proc?)
	// https://golangbyexample.com/print-struct-variables-golang/
	// fmt.Printf("%+v", visits)

	// https://medium.com/glottery/golang-and-mongodb-with-go-mongo-driver-part-1-1c43aba25a1
	docs := []interface{}{}
	// docs = append(docs, visits...) geht leider nicht
	for _, v := range visits {
		docs = append(docs, v)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	res, err := t.collection.InsertMany(ctx, docs)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(res.InsertedIDs)

	// 4. Delete processed data in Redis
	for _, key := range allKeys {
		_, err := t.redisClient.Del(ctx, key).Result()
		if err != nil {
			fmt.Println(err) // ToDO: Log
			return           // abort entire function in case of an error Altenrative: noch eine ArrayListe bauen mit "korrekten"
		}
	}
}

// internal methods used by multiple functions

// get a list of keys matching a specific name
// ToDO: evtl. in redis/db package auslagern
func (t *Tracker) getKeys(searchMask string) ([]string, error) {

	var ctx = context.Background()
	var cursor uint64
	var err error

	var keys []string // current iteration of cursor
	var allKeys []string

	for {
		keys, cursor, err = t.redisClient.Scan(ctx, cursor, searchMask, 10).Result()
		if err != nil {
			return nil, helpers.WrapError(err, helpers.FuncName())
		}

		// jede cursor iteration in die gesamtliste übernehmen
		allKeys = append(allKeys, keys...)

		/*
			// spread operator entspricht dem hier
			for _, v := range keys {
				allKeys = append(allKeys, v)
			}
		*/

		// loop beenden wenn nichts mehr kommt
		if cursor == 0 {
			break
		}
	}
	return allKeys, nil
}
