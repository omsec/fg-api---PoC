package models

import (
	"context"
	"fmt"
	"forza-garage/database"
	"forza-garage/helpers"
	"forza-garage/lookups"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// User is the "interface" used for client communication
type User struct {
	ID           primitive.ObjectID `json:"id" bson:"_id"`
	LoginName    string             `json:"loginName" bson:"loginName"` // unique
	Password     string             `json:"password" bson:"password"`   // hash value
	RoleCode     int32              `json:"roleCode" bson:"roleCD"`
	RoleText     string             `json:"roleText" bson:"-"`
	LanguageCode int32              `json:"languageCode" bson:"languageCD" header:"Language"`
	LanguageText string             `json:"languageText" bson:"-"`
	EMailAddress string             `json:"eMail" bson:"eMail"`     // unique
	XBoxTag      string             `json:"XBoxTag" bson:"XBoxTag"` // unique
	LastSeenTS   time.Time          `json:"lastSeenTS" bson:"lastSeenTS,omitempty"`
	Friends      []UserRef          `json:"friends" bson:"friends,omitempty"`
	// ToDo: Folloers evtl. in anderer Collection, wenn Array zu gross wird
	// Following []UserRef
	// Followers []UserRef
	// ToDo: []LastPasswords - check for 90 days or 10 entries
}

// Credentials is used for programmatic control
// non-ptr values require annotations!
type Credentials struct {
	UserID       primitive.ObjectID
	LoginName    string
	RoleCode     int32 `bson:"roleCD"`
	LanguageCode int32 `bson:"languageCD"` // ToDo: Lesen aus Header für ANONYM, DB für Members (override-Möglichkeit)
	Friends      []UserRef
}

// UserRef is a simple reference to user infromation, eg. uses in the friendlist
type UserRef struct {
	ID        primitive.ObjectID `json:"id" bson:"_id"`
	LoginName string             `json:"loginName" bson:"loginName"`
}

// UserModel provides the logic to the interface and access to the database
type UserModel struct {
	Client     *mongo.Client
	Collection *mongo.Collection
}

// UserExists checks if a User Name is available - used in client for in-type error checking
// (wrapper of internal helper function)
func (m UserModel) UserExists(userName string) bool {
	b, _ := userExists(m.Collection, userName)
	return b
}

// EMailAddressExists checks if an eMail-Address is already assigned with any User Name
// used in client for in-type error checking
func (m UserModel) EMailAddressExists(emailAddress string) bool {
	b, _ := eMailExists(m.Collection, emailAddress)
	return b
}

// CreateUser adds a new User
func (m UserModel) CreateUser(user User) (string, error) {

	var err error

	// ToDO: Validate (ext fnc)

	// ToDo: entfernen => PK in DB machen
	// braucht Hilfs-Proc (package DB) um die MSG zu parsen key: ....
	// "multiple write errors: [{write errors: [{E11000 duplicate key error collection: forzaGarage.users index: loginName_1 dup key: { loginName: \"roger\" }}]}, {<nil>}]"
	// https://stackoverflow.com/questions/56916969/with-mongodb-go-driver-how-do-i-get-the-inner-exceptions
	b, err := userExists(m.Collection, user.LoginName)
	if b || err != nil {
		return "", ErrUserNameNotAvailable
	}

	// ToDo: entfernen => PK in DB machen
	b, err = eMailExists(m.Collection, user.EMailAddress)
	if b || err != nil {
		return "", ErrEMailAddressTaken
	}

	pwdHash, err := helpers.GenerateHash(user.Password)
	if err != nil {
		return "", helpers.WrapError(err, helpers.FuncName())
	}

	user.ID = primitive.NewObjectID()
	user.Password = pwdHash
	user.RoleCode = lookups.UserRoleGuest
	user.LastSeenTS = time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	res, err := m.Collection.InsertOne(ctx, user)
	if err != nil {
		return "", helpers.WrapError(err, helpers.FuncName()) // primitive.NilObjectID.Hex() ? probly useless
	}

	return res.InsertedID.(primitive.ObjectID).Hex(), nil
}

// GetUserByName reads a user's login account data
func (m UserModel) GetUserByName(userName string) (*User, error) {

	var err error
	var user User

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	err = m.Collection.FindOne(ctx, bson.M{"loginName": userName}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrInvalidUser
		}
		// pass any other error
		return nil, helpers.WrapError(err, helpers.FuncName())
	}

	// add look-up texts
	addLookups(&user)

	return &user, nil
}

// GetUserByID reads a user's login account data
func (m UserModel) GetUserByID(ID string) (*User, error) {

	var user User

	// https://ildar.pro/golang-hints-create-mongodb-object-id-from-string/
	id, err := primitive.ObjectIDFromHex(ID)
	if err != nil {
		return nil, ErrInvalidUser
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	err = m.Collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrInvalidUser
		}
		// pass any other error
		return nil, helpers.WrapError(err, helpers.FuncName())
	}

	// add look-up text
	//user.RoleText = database.GetLookupText(lookups.LookupType(lookups.LTuserRole), user.RoleCode)
	addLookups(&user)

	return &user, nil
}

// GetUserName returns the login name from an ID (reduced version, without profile data)
func (m UserModel) GetUserName(ID string) (string, error) {

	data := struct {
		//ID        primitive.ObjectID `bson:"_id"`
		LoginName string `bson:"loginName"`
	}{}

	id, err := primitive.ObjectIDFromHex(ID)
	if err != nil {
		return "", ErrInvalidUser
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	fields := bson.D{
		{Key: "_id", Value: 0}, // _id kommt immer, ausser es wird explizit ausgeschlossen (0)
		{Key: "loginName", Value: 1}}

	err = m.Collection.FindOne(ctx, bson.M{"_id": id}, options.FindOne().SetProjection(fields)).Decode(&data)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return "", ErrInvalidUser
		}
		// pass any other error
		return "", helpers.WrapError(err, helpers.FuncName())

	}

	return data.LoginName, nil
}

// CheckPassword tests if a login's password matches
// (kein DB-Zugriff nötig)
func (m UserModel) CheckPassword(givenPassword string, userInfo User) bool {
	match, err := helpers.CompareHash(userInfo.Password, givenPassword)
	if err != nil {
		return false
	}
	return match
}

// SetLastSeen saves timestamp of last log-in
// ToDo: add IP-Address & record history (collection analytics)
// ToDO: auch in refresh rufen
func (m UserModel) SetLastSeen(userID primitive.ObjectID) {
	// no error is returned since this function is not essential

	filter := bson.D{{Key: "_id", Value: userID}}
	update := bson.D{{Key: "$set", Value: bson.D{{Key: "lastSeenTS", Value: time.Now()}}}}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	// just fire & forget
	_, _ = m.Collection.UpdateOne(ctx, filter, update)
}

// SetPassword is used to change a User's password
func (m UserModel) SetPassword(userID primitive.ObjectID, newPassword string) error {
	// ToDO: call PWD-Validator func

	pwdHash, err := helpers.GenerateHash(newPassword)
	if err != nil {
		return helpers.WrapError(err, helpers.FuncName())
	}

	filter := bson.D{{Key: "_id", Value: userID}}
	update := bson.D{{Key: "$set", Value: bson.D{{Key: "password", Value: pwdHash}}}}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	result, err := m.Collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return helpers.WrapError(err, helpers.FuncName())
	}

	// just an additional check to discover data consistency problems
	if result.MatchedCount != 1 || result.ModifiedCount != 1 {
		// treat this as system error (which causes 500)
		return helpers.WrapError(ErrMultipleRecords, helpers.FuncName())
	}

	return nil
}

// GetCredentials returns account infos to control permissions and text-out (language)
func (m UserModel) GetCredentials(UserID string) (*Credentials, error) {
	var credentials Credentials

	if UserID == "" {
		credentials.UserID = primitive.NilObjectID
		// anonymous (visitor) receives default role
		credentials.RoleCode = lookups.UserRoleGuest
	} else {
		// look-up credentials in database

		// https://ildar.pro/golang-hints-create-mongodb-object-id-from-string/
		id, err := primitive.ObjectIDFromHex(UserID)
		if err != nil {
			return nil, ErrInvalidUser
		}

		fields := bson.D{
			{Key: "_id", Value: 0}, // _id kommt immer, ausser es wird explizit ausgeschlossen (0)
			{Key: "loginName", Value: 1},
			{Key: "roleCD", Value: 1}, // {Key: "metaInfo.rating", Value: 1}, -- so könnte die nested struct eingeschränkt werden
			{Key: "languageCD", Value: 1},
			{Key: "friends", Value: 1},
		}

		opts := options.FindOne().SetProjection(fields)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel() // nach 10 Sekunden abbrechen

		err = m.Collection.FindOne(ctx, bson.M{"_id": id}, opts).Decode(&credentials)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return nil, ErrInvalidUser
			}
			// pass any other error
			return nil, helpers.WrapError(err, helpers.FuncName())
		}
		credentials.UserID = id
	}

	return &credentials, nil
}

// AddFriend adds another user to the friendlist
func (m UserModel) AddFriend(targetUserID string, friendUserID string) error {

	// ToDo:
	// auch die gegenrichtung erstellen (add target to friend)

	// ToDO: Prüfung entfernen bei mehreren; im Loop einfach ignorieren ohne FEhler
	// überhaupt nötig? hängt vom gui ab
	if targetUserID == friendUserID {
		return ErrInvalidFriend
	}

	// objectID required for update
	targetID, err := primitive.ObjectIDFromHex(targetUserID)
	if err != nil {
		return ErrInvalidUser
	}

	friendID, err := primitive.ObjectIDFromHex(friendUserID)
	if err != nil {
		return ErrInvalidUser
	}

	friendInfo, err := m.GetCredentials(friendUserID)
	if err != nil {
		return err
	}

	friend := UserRef{
		ID:        friendID,
		LoginName: friendInfo.LoginName}

	filter := bson.D{{Key: "_id", Value: targetID}}
	// $addToSet silently checks for duplicates - $push does not
	update := bson.D{{Key: "$addToSet", Value: bson.D{{Key: "friends", Value: friend}}}}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	// not interessted in result (eg. no of changes)
	_, err = m.Collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return helpers.WrapError(err, helpers.FuncName())
	}

	return nil
}

// RemoveFriend adds another user to the friendlist
func (m UserModel) RemoveFriend(targetUserID string, friendUserID string) error {

	// auch die gegenrichtung löschen (remove target from friend)

	// ToDO: Prüfung entfernen bei mehreren; im Loop einfach ignorieren ohne FEhler
	// überhaupt nötig? hängt vom gui ab
	if targetUserID == friendUserID {
		return ErrInvalidFriend
	}

	// objectID required for update
	targetID, err := primitive.ObjectIDFromHex(targetUserID)
	if err != nil {
		return ErrInvalidUser
	}

	friendID, err := primitive.ObjectIDFromHex(friendUserID)
	if err != nil {
		return ErrInvalidUser
	}

	filter := bson.D{{Key: "_id", Value: targetID}}
	// Formatierung ausprobieren was lesbarer scheint - mit Zeilenumbruch braucht's Kommas...
	// update := bson.D{{Key: "$pull", Value: bson.D{{Key: "friends", Value: bson.D{{Key: "_id", Value: friendID}}}}}}
	update := bson.D{
		{Key: "$pull", Value: bson.D{
			{Key: "friends", Value: bson.D{
				{Key: "_id", Value: friendID}},
			}},
		}}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	// not interessted in result (eg. no of changes)
	res, err := m.Collection.UpdateOne(ctx, filter, update)
	if err != nil {
		// fmt.Println(err)
		return helpers.WrapError(err, helpers.FuncName())
	}

	fmt.Println(res)

	return nil
}

// public "static" methods

// UserReferenced scans a slice for a given item
func UserReferenced(slice []UserRef, val primitive.ObjectID) bool {
	for _, item := range slice {
		if item.ID == val {
			return true
		}
	}
	return false
}

// GrantPermissions enforces access rights
func GrantPermissions(itemVisibilityCode int32, itemCreatorID primitive.ObjectID, credentials *Credentials) error {

	if credentials.RoleCode == lookups.UserRoleAdmin {
		return nil
	}

	if itemVisibilityCode == lookups.VisibilityMembers && credentials.RoleCode == lookups.UserRoleGuest {
		// get a log-in and make friends
		return ErrGuest
	}

	if (itemVisibilityCode == lookups.VisibilityMembers) && (UserReferenced(credentials.Friends, itemCreatorID) == false) && (itemCreatorID != credentials.UserID) {
		// make friends with them
		return ErrNotFriend
	}

	if itemVisibilityCode == lookups.VisibilityNone && (credentials.UserID != itemCreatorID) {
		// ask them to share
		return ErrPrivate
	}

	// all checks passed
	return nil
}

// internal implementations that are used by multiple methods of the model and corresponding handlers
// ToDo: müsste nicht ausgelagert sein, kann private member sein (klein schreiben)

func userExists(collection *mongo.Collection, userName string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	// there seems to be no function like "exists" so a projection on just the ID is used
	fields := bson.D{
		{Key: "_id", Value: 1}}

	data := struct {
		ID primitive.ObjectID `bson:"_id"`
	}{}

	// some (old) sources say FindOne is slow and we should use find instead... (?)
	err := collection.FindOne(ctx, bson.M{"loginName": userName}, options.FindOne().SetProjection(fields)).Decode(&data)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
		// treat errors as a "yes" - caller should not evaluate the result in case of an error
		return true, helpers.WrapError(err, helpers.FuncName())
	}
	// no error means a document was found, hence the user does exist
	return true, nil
}

func eMailExists(collection *mongo.Collection, emailAddress string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	// there seems to be no function like "exists" so a projection on just the ID is used
	fields := bson.D{
		{Key: "_id", Value: 1}}

	data := struct {
		ID primitive.ObjectID `bson:"_id"`
	}{}

	// some (old) sources say FindOne is slow and we should use find instead... (?)
	// ToDO: Add index to field in MongoDB
	err := collection.FindOne(ctx, bson.M{"eMail": emailAddress}, options.FindOne().SetProjection(fields)).Decode(&data)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
		// treat errors as a "yes" - caller should not evaluate the result in case of an error
		return true, helpers.WrapError(err, helpers.FuncName())
	}
	// no error means a document was found, hence the user does exist
	return true, nil
}

// internal helpers

// actually that's not immutable, but ok here
func addLookups(user *User) *User {
	user.RoleText = database.GetLookupText(lookups.LookupType(lookups.LTuserRole), user.RoleCode)
	user.LanguageText = database.GetLookupText(lookups.LookupType(lookups.LTlang), user.LanguageCode)

	return user
}
