package models

import (
	"context"
	"forza-garage/database"
	"forza-garage/helpers"
	"forza-garage/lookups"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ToDo: UI on Forza Sharing Code
// https://docs.mongodb.com/manual/core/index-unique/

// Course is the "interface" used for client communication
type Course struct {
	ID             primitive.ObjectID `json:"id" bson:"_id"`
	MetaInfo       Header             `json:"metaInfo" bson:"metaInfo"` // non-ptr = always present
	VisibilityCode int32              `json:"visibilityCode" bson:"visibilityCD"`
	VisibilityText string             `json:"visibilityText" bson:"-"`
	GameCode       int32              `json:"gameCode" bson:"gameCD"`
	GameText       string             `json:"gameText" bson:"-"`
	TypeCode       int32              `json:"typeCode" bson:"courseTypeCD"` // identifies object type (for searches, $exists)
	TypeText       string             `json:"typeText" bson:"-"`
	ForzaSharing   int32              `json:"forzaSharing" bson:"forzaSharing"`
	Name           string             `json:"name" bson:"name"` // same name as CMPs to enables over-all searches
	SeriesCode     int32              `json:"seriesCode" bson:"seriesCD"`
	SeriesText     string             `json:"seriesText" bson:"-"`
	CarClassesCode []int32            `json:"carClassesCode" bson:"carClassesCD"`
	CarClassesText []string           `json:"carClassesText" bson:"-"`
	Route          *CourseRef         `json:"route" bson:"route,omitempty"` // standard route which a custom route is based on
}

// CourseRef is used as a reference
type CourseRef struct {
	ID   primitive.ObjectID `json:"id" bson:"_id"`
	Name string             `json:"name" bson:"name"`
}

// CourseListItem is the reduced/simplified model used for listings
type CourseListItem struct {
	ID             primitive.ObjectID `json:"id"`
	CreatedTS      time.Time          `json:"createdTS"`
	CreatedID      primitive.ObjectID `json:"createdID"`
	CreatedName    string             `json:"createdName"`
	Rating         float32            `json:"rating"`
	GameCode       int32              `json:"gameCode"`
	GameText       string             `json:"gameText"`
	Name           string             `json:"name"`
	ForzaSharing   int32              `json:"forzaSharing"`
	SeriesCode     int32              `json:"seriesCode"`
	SeriesText     string             `json:"seriesText"`
	CarClassesCode []int32            `json:"carClassesCode"`
	CarClassesText []string           `json:"carClassesText"`
}

// CourseSearch is passed as the search params // ToDO: evt. CourseSearchParams nennen, searchMode integrieren
type CourseSearch struct {
	// ToDo: UserID (ObjectID) in Credentials verschieben (auch für get etc. benutzen)
	GameText    string // client should pass readable text in URL rather than codes
	SearchTerm  string
	Credentials *Credentials
}

// CourseModel provides the logic to the interface and access to the database
type CourseModel struct {
	Client     *mongo.Client
	Collection *mongo.Collection
}

// Validate checks given values and sets defaults where applicable (immutable)
func (m CourseModel) Validate(course Course) (*Course, error) {

	cleaned := course

	// ToDo:
	// Clean Strings
	// Validate Code Values (?) -> dann geht es nicxht mit Const/Enum, sondern const-array
	// ..according to model
	// Forza Share Code (Range)

	cleaned.Name = strings.TrimSpace(cleaned.Name)
	if course.Name == "" {
		return nil, ErrCourseNameMissing
	}

	return &cleaned, nil
}

// ForzaSharingExists checks if a "Sharing Code" in the game already exists (which is their PK)
// this is used for in-line validation while typing in the client's form
func (m CourseModel) ForzaSharingExists(sharingCode int32) (bool, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	// there seems to be no function like "exists" so a projection on just the ID is used
	fields := bson.D{
		{Key: "_id", Value: 1}}

	data := struct {
		ID primitive.ObjectID `bson:"_id"`
	}{}

	err := m.Collection.FindOne(ctx, bson.M{"forzaSharing": sharingCode}, options.FindOne().SetProjection(fields)).Decode(&data)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
		// treat errors as a "yes" - caller should not evaluate the result in case of an error
		return true, helpers.WrapError(err, helpers.FuncName())
	}
	// no error means a document was found, hence the object exists
	return true, nil
}

// CreateCourse adds a new route - validated by controller
// ToDO: Rename "Add" ?
func (m CourseModel) CreateCourse(course *Course) (string, error) {

	// set "system-fields"
	course.ID = primitive.NewObjectID()
	course.MetaInfo.TouchedTS = time.Now()
	course.MetaInfo.Rating = 0
	course.MetaInfo.RecVer = 0
	course.TypeCode = lookups.CourseTypeCustom

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	res, err := m.Collection.InsertOne(ctx, course)
	if err != nil {
		// leider können DB-Error Codes nicht direkt aus dem Fehler ausgelesen werden
		// https://stackoverflow.com/questions/56916969/with-mongodb-go-driver-how-do-i-get-the-inner-exceptions

		if (err.(mongo.WriteException).WriteErrors[0].Code) == 11000 {
			// Error 11000 = DUP
			// since there is only one unique index in the collection, it's a duplicate forza share code
			return "", ErrForzaSharingCodeTaken
		}
		// any other error
		return "", helpers.WrapError(err, helpers.FuncName()) // primitive.NilObjectID.Hex() ? probly useless
	}

	return res.InsertedID.(primitive.ObjectID).Hex(), nil
}

// SearchCourses lists or searches course (ohne Comments, aber mit Files/Tags)
func (m CourseModel) SearchCourses(searchSpecs *CourseSearch) ([]CourseListItem, error) {

	// Verkleinerte/vereinfachte Struktur für Listen
	// MongoDB muss eine passende Struktur erhalten um die Daten aufzunehmen (z. B. mit nested Arrays)
	// das API kann die Daten dann in die Listenstruktur kopieren
	// daher wird zum Aufnehmen der Daten aus der DB immer mit der Original-Struktur gearbeitet
	// Speicherbedarf bleibt halt gleich, dafür nimmt die Netzlast ab

	// use original struct to receive selected fields
	fields := bson.D{
		{Key: "_id", Value: 1},      // _id kommt immer, ausser es wird explizit ausgeschlossen (0)
		{Key: "metaInfo", Value: 1}, // {Key: "metaInfo.rating", Value: 1}, -- so könnte die nested struct eingeschränkt werden
		{Key: "gameCD", Value: 1},
		{Key: "name", Value: 1},
		{Key: "forzaSharing", Value: 1},
		{Key: "seriesCD", Value: 1},
		{Key: "carClassesCD", Value: 1},
	}

	sort := bson.D{
		{Key: "metaInfo.rating", Value: -1},
		{Key: "metaInfo.touchedTS", Value: -1},
	}

	opts := options.Find().SetProjection(fields).SetLimit(20).SetSort(sort)

	// https://docs.mongodb.com/manual/tutorial/query-documents/
	// https://docs.mongodb.com/manual/reference/operator/query/#query-selectors
	// https://stackoverflow.com/questions/3305561/how-to-query-mongodb-with-like

	gameCode, err := database.GetLookupValue(lookups.LookupType(lookups.LTgame), searchSpecs.GameText)
	if err != nil {
		gameCode = lookups.GameFH4
	}

	// perhaps, the searchTerm is a forza share code
	i, _ := strconv.Atoi(searchSpecs.SearchTerm)

	// construct a document containing the search parameters
	filter := bson.D{}

	if searchSpecs.Credentials.RoleCode == lookups.UserRoleGuest {
		// anonymous visitors will only receive PUBLIC routes
		if searchSpecs.SearchTerm == "" {
			filter = bson.D{
				// every next field is AND
				{Key: "gameCD", Value: gameCode},                                      // $eq kann wegelassen werden
				{Key: "courseTypeCD", Value: bson.D{{Key: "$exists", Value: "true"}}}, // return std and community courses
				{Key: "visibilityCD", Value: lookups.VisibilityAll},
			}
		} else {
			filter = bson.D{
				// every next field is AND
				{Key: "gameCD", Value: gameCode},                                      // $eq kann wegelassen werden
				{Key: "courseTypeCD", Value: bson.D{{Key: "$exists", Value: "true"}}}, // return std and community courses
				{Key: "visibilityCD", Value: lookups.VisibilityAll},
				{Key: "$or", Value: bson.A{ // AND OR (...
					bson.D{{Key: "name", Value: primitive.Regex{Pattern: ".*" + searchSpecs.SearchTerm + ".*", Options: "/i"}}}, // LIKE %searchTerm% (case-insensitive)
					bson.D{{Key: "forzaSharing", Value: i}}, // 0 if searchTerm was alpha-numeric
				}},
			}
		}
	} else {
		// if a user is logged-in, check their privilidges (must be Admin or Member)
		//fmt.Printf("%s is logged in with role %v", searchSpecs.Credentials.LoginName, searchSpecs.Credentials.RoleCode)
		if searchSpecs.Credentials.RoleCode == lookups.UserRoleAdmin {
			// no visibility check needed for admins
			if searchSpecs.SearchTerm == "" {
				filter = bson.D{
					// every next field is AND
					{Key: "gameCD", Value: gameCode},                                      // $eq kann wegelassen werden
					{Key: "courseTypeCD", Value: bson.D{{Key: "$exists", Value: "true"}}}, // return std and community courses
					// visibility check removed
				}
			} else {
				// apply search Term
				filter = bson.D{
					// every next field is AND
					{Key: "gameCD", Value: gameCode},                                      // $eq kann wegelassen werden
					{Key: "courseTypeCD", Value: bson.D{{Key: "$exists", Value: "true"}}}, // return std and community courses
					// visibility check removed
					{Key: "$or", Value: bson.A{ // AND OR (...
						bson.D{{Key: "name", Value: primitive.Regex{Pattern: ".*" + searchSpecs.SearchTerm + ".*", Options: "/i"}}}, // LIKE %searchTerm% (case-insensitive)
						bson.D{{Key: "forzaSharing", Value: i}}, // 0 if searchTerm was alpha-numeric
					}},
				}
			}
		} else {
			// check visibility
			friendIDs := make([]primitive.ObjectID, len(searchSpecs.Credentials.Friends))
			for i, friend := range searchSpecs.Credentials.Friends {
				friendIDs[i] = friend.ID
			}

			if searchSpecs.SearchTerm == "" {
				filter = bson.D{
					// every next field is AND
					{Key: "gameCD", Value: gameCode},                                      // $eq kann wegelassen werden
					{Key: "courseTypeCD", Value: bson.D{{Key: "$exists", Value: "true"}}}, // return std and community courses
					// visibility check
					{Key: "$or", Value: bson.A{
						bson.D{{Key: "visibilityCD", Value: 0}},
						bson.D{{Key: "metaInfo.createdID", Value: searchSpecs.Credentials.UserID}},
						bson.D{{Key: "$and", Value: bson.A{
							bson.D{{Key: "visibilityCD", Value: 1}},
							bson.D{{Key: "metaInfo.createdID", Value: bson.D{{Key: "$in", Value: friendIDs}}}}, // nested doc for $in
						}}}, // nested $and-array im $or
					}}, // $or-array
				}
			} else {
				filter = bson.D{
					// every next field is AND
					{Key: "gameCD", Value: gameCode},                                      // $eq kann wegelassen werden
					{Key: "courseTypeCD", Value: bson.D{{Key: "$exists", Value: "true"}}}, // return std and community courses
					// visibility check
					{Key: "$or", Value: bson.A{
						bson.D{{Key: "visibilityCD", Value: 0}},
						bson.D{{Key: "metaInfo.createdID", Value: searchSpecs.Credentials.UserID}},
						bson.D{{Key: "$and", Value: bson.A{
							bson.D{{Key: "visibilityCD", Value: 1}},
							bson.D{{Key: "metaInfo.createdID", Value: bson.D{{Key: "$in", Value: friendIDs}}}}, // nested doc for $in
						}}}, // nested $and-array im $or
					}}, // $or-array
					// apply search term
					{Key: "$or", Value: bson.A{ // AND OR (...
						bson.D{{Key: "name", Value: primitive.Regex{Pattern: ".*" + searchSpecs.SearchTerm + ".*", Options: "/i"}}}, // LIKE %searchTerm% (case-insensitive)
						bson.D{{Key: "forzaSharing", Value: i}}, // 0 if searchTerm was alpha-numeric
					}}, // $or-array
				}
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	cursor, err := m.Collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, helpers.WrapError(err, helpers.FuncName())
	}

	// receive results
	var courses []Course

	err = cursor.All(ctx, &courses)
	if err != nil {
		return nil, helpers.WrapError(err, helpers.FuncName())
	}

	// check for empty result set (no error raised by find)
	if courses == nil {
		return nil, ErrNoData
	}

	// copy data to reduced list-struct
	var courseList []CourseListItem
	var course CourseListItem

	for _, v := range courses {
		course.ID = v.ID
		course.CreatedTS = primitive.ObjectID.Timestamp(v.ID)
		course.CreatedID = v.MetaInfo.CreatedID
		course.CreatedName = v.MetaInfo.CreatedName
		course.Rating = v.MetaInfo.Rating
		course.GameCode = v.GameCode
		course.GameText = database.GetLookupText(lookups.LookupType(lookups.LTgame), v.GameCode)
		course.Name = v.Name
		course.ForzaSharing = v.ForzaSharing
		course.SeriesCode = v.SeriesCode
		course.SeriesText = database.GetLookupText(lookups.LookupType(lookups.LTseries), v.SeriesCode)
		course.CarClassesCode = v.CarClassesCode
		//course.CarClassText = database.GetLookupText(lookups.LookupType(lookups.LTcarClass), v.CarClassCode)
		course.CarClassesText = make([]string, len(v.CarClassesCode))
		for i := range v.CarClassesCode {
			course.CarClassesText[i] = database.GetLookupText(lookups.LookupType(lookups.LTcarClass), v.CarClassesCode[i])
		}

		courseList = append(courseList, course)
	}

	return courseList, nil
}

// GetCourse returns one
func (m CourseModel) GetCourse(courseID string, credentials *Credentials) (*Course, error) {

	id, err := primitive.ObjectIDFromHex(courseID)
	if err != nil {
		return nil, ErrNoData
	}

	data := Course{}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	// später vielleicht project() wenn's zu viele felder werden (excl. nested oder sowas)
	err = m.Collection.FindOne(ctx, bson.M{"_id": id}).Decode(&data)
	if err != nil {
		return nil, ErrNoData
	}

	err = GrantPermissions(data.VisibilityCode, data.MetaInfo.CreatedID, credentials)
	if err != nil {
		// no wrapping needed, since function returns app errors
		return nil, err
	}

	m.addLookups(&data)

	return &data, nil
}

// UpdateCourse modifies a given course
func (m CourseModel) UpdateCourse(course *Course, credentials *Credentials) error {

	// read "metadata" to check permissions and perform optimistic locking
	// könnte eigentlich ausgelagert werden
	fields := bson.D{
		{Key: "_id", Value: 0},
		{Key: "metaInfo.createdID", Value: 1},
		{Key: "metaInfo.recVer", Value: 1},
		{Key: "visibilityCD", Value: 1},
	}

	filter := bson.D{{Key: "_id", Value: course.ID}}

	data := struct {
		CreatedID      primitive.ObjectID `bson:"_id"`
		RecVer         int64              `bson:"recVer"`
		VisibilityCode int32              `bson:"visibilityCD"`
	}{}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	// alternative schreibweisen:
	//err := m.Collection.FindOne(ctx, bson.D{{Key: "_id", Value: course.ID}}, options.FindOne().SetProjection(fields)).Decode(&data)
	//err := m.Collection.FindOne(ctx, bson.M{"_id": course.ID}, options.FindOne().SetProjection(fields)).Decode(&data)

	err := m.Collection.FindOne(ctx, filter, options.FindOne().SetProjection(fields)).Decode(&data)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return ErrNoData // document might have been deleted
		}
		// pass any other error
		return helpers.WrapError(err, helpers.FuncName())
	}

	if (data.CreatedID != credentials.UserID) && (credentials.RoleCode < lookups.UserRoleAdmin) {
		return ErrDenied
	}

	err = GrantPermissions(data.VisibilityCode, data.CreatedID, credentials)
	if err != nil {
		// no wrapping needed, since function returns app errors
		return err
	}

	if data.RecVer != course.MetaInfo.RecVer {
		// document was changed by another user since last read
		return ErrRecordChanged
	}

	// ToDO: einzel-upd wohl besser, oder gar replace?
	// replace nicht "nachhaltig" wenn bspw. Arrays/Nesteds drin sind, die gar nicht immer gelesen werden
	// definition für den moment: "alle änderbaren" felder halt neu setzen
	// arrays somit ersetzen, oder in spez. services ändern (z. B. add friends, falls embedded)

	// set "systemfields"
	course.MetaInfo.ModifiedID = credentials.UserID
	course.MetaInfo.ModifiedName = credentials.LoginName
	course.MetaInfo.ModifiedTS = time.Now()
	course.MetaInfo.TouchedTS = course.MetaInfo.ModifiedTS

	// set fields to be possibily updated
	fields = bson.D{
		// systemfields
		{Key: "$set", Value: bson.D{{Key: "metaInfo.modifiedTS", Value: course.MetaInfo.ModifiedTS}}},
		{Key: "$set", Value: bson.D{{Key: "metaInfo.modifiedID", Value: course.MetaInfo.ModifiedID}}},
		{Key: "$set", Value: bson.D{{Key: "metaInfo.modifiedName", Value: course.MetaInfo.ModifiedName}}},
		{Key: "$set", Value: bson.D{{Key: "metaInfo.touchedTS", Value: course.MetaInfo.TouchedTS}}},
		{Key: "$inc", Value: bson.D{{Key: "metaInfo.recVer", Value: 1}}},
		// payload
		{Key: "$set", Value: bson.D{{Key: "visibilityCD", Value: course.VisibilityCode}}},
		{Key: "$set", Value: bson.D{{Key: "gameCD", Value: course.GameCode}}},
		// typeCode is static
		{Key: "$set", Value: bson.D{{Key: "forzaSharing", Value: course.ForzaSharing}}},
		{Key: "$set", Value: bson.D{{Key: "name", Value: course.Name}}},
		{Key: "$set", Value: bson.D{{Key: "seriesCD", Value: course.SeriesCode}}},
		{Key: "$set", Value: bson.D{{Key: "carClassesCD", Value: course.CarClassesCode}}},
	}

	result, err := m.Collection.UpdateOne(ctx, filter, fields)
	if err != nil {
		return helpers.WrapError(err, helpers.FuncName())
	}

	if result.MatchedCount == 0 {
		return ErrNoData // document might have been deleted
	}

	// ToDO: überlegen - rückgsabewerte sinnvoll? (z. B. timestamp? oder die ID analog add?)
	return nil
}

// internal helpers (private methods)

// actually that's not immutable, but ok here
func (m CourseModel) addLookups(course *Course) *Course {
	course.VisibilityText = database.GetLookupText(lookups.LookupType(lookups.LTvisibility), course.VisibilityCode)
	course.GameText = database.GetLookupText(lookups.LookupType(lookups.LTgame), course.GameCode)
	course.TypeText = database.GetLookupText(lookups.LookupType(lookups.LTcourseType), course.TypeCode)
	course.SeriesText = database.GetLookupText(lookups.LookupType(lookups.LTseries), course.SeriesCode)
	// course.CarClassText = database.GetLookupText(lookups.LookupType(lookups.LTcarClass), course.CarClassCode)
	course.CarClassesText = make([]string, len(course.CarClassesCode))
	for i := range course.CarClassesCode {
		course.CarClassesText[i] = database.GetLookupText(lookups.LookupType(lookups.LTcarClass), course.CarClassesCode[i])
	}

	return course
}
