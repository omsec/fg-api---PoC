package models

import (
	"context"
	"fmt"
	"forza-garage/apperror"
	"forza-garage/database"
	"forza-garage/helpers"
	"forza-garage/lookups"
	"os"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// File Name Convention at Destination:
// otype_uuid.ext

// FileInfo is what's embedded in profiles and returned to the client
type FileInfo struct {
	URL         string `json:"url"` // built by controller from SysFileName
	Description string `json:"description,omitempty"`
	StatusCode  int32  `json:"statusCode"`
	StatusText  string `json:"statusText"`
}

// API-internal data structures
// UploadHeader is the Document which belongs to a Profile
type UploadHeader = struct {
	ID          primitive.ObjectID `json:"-" bson:"_id"`
	ProfileID   primitive.ObjectID `json:"-" bson:"profileID"` // passed via form
	ProfileType string             `json:"-" bson:"profileType"`
	Slots       []Slot             `json:"-" bson:"slots"`
}

// Slot holds a file's metadata
// it's used to keep the current/old file while a new is under review
type Slot = struct {
	Staged *UploadInfo `json:"-" bson:"staged,omitempty"`
	Active *UploadInfo `json:"-" bson:"active,omitempty"`
}

// UploadInfo contains the meta data of an uploaded file
type UploadInfo = struct {
	UploadedTS   time.Time           `json:"uploadedTS" bson:"-"`
	UploadedID   primitive.ObjectID  `json:"uploadedID" bson:"uploadedID"`
	UploadedName string              `json:"uploadedName" bson:"uploadedName"`
	SysFileName  string              `json:"-" bson:"fileName"`            // internal server file name
	OrigFileName string              `json:"fileName" bson:"origFileName"` // shown to client (uploader)
	Description  string              `json:"description" bson:"description,omitempty"`
	StatusCode   int32               `json:"statusCode" bson:"statusCD"` // will be using same code/status model as comments
	StatusText   string              `json:"statusText" bson:"-"`
	StatusTS     time.Time           `json:"statusTS" bson:"statusTS"`
	StatusID     *primitive.ObjectID `json:"statusID" bson:"statusID,omitempty"`     // not set for system
	StatusName   *string             `json:"statusName" bson:"statusName,omitempty"` // not set for system
	URL          string              `json:"url" bson:"-"`
}

/*
// UploadData is what's sent to the client (eg. Profile Picture or Screenshots)
// the structure is usually initialized by a model function, called by GetXXX
type UploadData = struct {
	URL        string `json:"URL"`
	StatusCode int32  `json:"statusCode"` // will be using same code/status model as comments
	StatusText string `json:"statusText"`
}
*/

// UploadModel provides the logic to the interface and access to the database
type UploadModel struct {
	Client     *mongo.Client
	Collection *mongo.Collection
	// Gewisse Informationen kommen vom User-Model, die werden hier referenziert
	// somit muss das nicht der Controller machen
	GetUserNameOID func(userID primitive.ObjectID) (string, error)
	GetCredentials func(userId string, loadFriendlist bool) *Credentials
	GetUserVote    func(profileID string, userID string) (int32, error) // injected from vote model
}

func (m UploadModel) SaveMetaData(profileID string, profileType string, uploadInfo *UploadInfo) error {

	profileOID := helpers.ObjectID(profileID)

	// ToDO: .env max_num of attachments per profile and check that

	// add more metadata
	now := time.Now()
	uploadInfo.UploadedTS = now
	uploadInfo.UploadedName, _ = m.GetUserNameOID(uploadInfo.UploadedID)
	uploadInfo.StatusTS = now
	// status depends on moderation feature toggle and is set in IF block below

	// MongoDB's upsert operation can not be used here, because additional entries will go into an array
	// that's why a "DocumentExists" function was created to manually check for existance
	exists, err := m.uploadsExists(profileOID)
	if err != nil {
		return err
	}

	if exists {
		// update slots array

		// user profiles are handled differently because there can be only one avatar
		// which is always saved in slot 0
		if profileType == "user" {
			var fields bson.D
			filter := bson.D{{Key: "profileID", Value: profileOID}}

			// profile pictures might be replaced, while generic uploads/screenshots may be removed.
			// hence the "old" (active/pending) file's name must be retrieved before updating the metadata,
			// so that file can be deleted
			fields = bson.D{
				{Key: "_id", Value: 0},
				{Key: "slots", Value: 1},
			}

			data := struct {
				Slots []Slot `bson:"slots"`
			}{}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel() // nach 10 Sekunden abbrechen

			err := m.Collection.FindOne(ctx, filter, options.FindOne().SetProjection(fields)).Decode(&data)
			if err != nil {
				return apperror.ErrNoData
			}

			// if moderation is enabled, the current profile picture will be left intact until the new one is approved
			if os.Getenv("UPLOAD_MODERATION") == "YES" {
				uploadInfo.StatusCode = lookups.CommentStatusPending
				fields = bson.D{
					{Key: "$set", Value: bson.D{{Key: "slots.0.active", Value: &uploadInfo}}},
				}
			} else {
				uploadInfo.StatusCode = lookups.CommentStatusVisible
				fields = bson.D{
					{Key: "$set", Value: bson.D{{Key: "slots.0.active", Value: &uploadInfo}}},
				}
			}
			uploadInfo.StatusText = database.GetLookupText(lookups.LookupType(lookups.LTcommentStatus), uploadInfo.StatusCode)

			ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel() // nach 10 Sekunden abbrechen

			result, err := m.Collection.UpdateOne(ctx, filter, fields)
			if err != nil {
				return helpers.WrapError(err, helpers.FuncName())
			}

			if result.MatchedCount == 0 {
				return apperror.ErrNoData // document might have been deleted
			}

			// delete the old file right away if everything was okay
			oldFile := ""
			if os.Getenv("UPLOAD_MODERATION") == "YES" {
				oldFile = os.Getenv("UPLOAD_TARGET") + "/" + data.Slots[0].Staged.SysFileName
			} else {
				oldFile = os.Getenv("UPLOAD_TARGET") + "/" + data.Slots[0].Active.SysFileName
			}
			err = os.Remove(oldFile)
			if err != nil {
				// ToDO: log
				fmt.Println(err)
			}

			return nil
		} else {
			// add additional file to slot

			var fields bson.D
			filter := bson.D{{Key: "profileID", Value: profileOID}}

			// check the length of the array and compare with limit
			fields = bson.D{
				{Key: "_id", Value: 0},
				{Key: "slots", Value: 1},
			}

			data := struct {
				Slots []Slot `bson:"slots"`
			}{}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel() // nach 10 Sekunden abbrechen

			err := m.Collection.FindOne(ctx, filter, options.FindOne().SetProjection(fields)).Decode(&data)
			if err != nil {
				return apperror.ErrNoData // internal data error
			}

			maxFiles, err := strconv.Atoi(os.Getenv("UPLOAD_MAX_FILES"))
			if err != nil {
				// ToDO: Log/Panic: Invalid COnfig
				maxFiles = 5
			}
			if len(data.Slots) > maxFiles {
				return ErrMaximumFilesReached
			}

			// if moderation is enabled, the current profile file will be left intact until the new one is approved
			if os.Getenv("UPLOAD_MODERATION") == "YES" {
				uploadInfo.StatusCode = lookups.CommentStatusPending
				fields = bson.D{
					{Key: "$push", Value: bson.D{
						{Key: "slots", Value: bson.D{
							{Key: "staged", Value: uploadInfo},
						}},
					}},
				}
			} else {
				uploadInfo.StatusCode = lookups.CommentStatusVisible
				fields = bson.D{
					{Key: "$push", Value: bson.D{
						{Key: "slots", Value: bson.D{
							{Key: "active", Value: uploadInfo},
						}},
					}},
				}
			}
			uploadInfo.StatusText = database.GetLookupText(lookups.LookupType(lookups.LTcommentStatus), uploadInfo.StatusCode)

			ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel() // nach 10 Sekunden abbrechen

			result, err := m.Collection.UpdateOne(ctx, filter, fields)
			if err != nil {
				return helpers.WrapError(err, helpers.FuncName())
			}

			if result.MatchedCount == 0 {
				return apperror.ErrNoData // document might have been deleted
			}

			return nil
		}

	} else {
		// create a new document
		var metaData UploadHeader
		metaData.ID = primitive.NewObjectID()
		metaData.ProfileID = profileOID
		metaData.ProfileType = profileType
		metaData.Slots = make([]Slot, 1)

		// if moderation is enabled, the current profile picture will be left intact until the new one is approved
		if os.Getenv("UPLOAD_MODERATION") == "YES" {
			uploadInfo.StatusCode = lookups.CommentStatusPending
			metaData.Slots[0].Staged = uploadInfo
		} else {
			uploadInfo.StatusCode = lookups.CommentStatusVisible
			metaData.Slots[0].Active = uploadInfo
		}
		uploadInfo.StatusText = database.GetLookupText(lookups.LookupType(lookups.LTcommentStatus), uploadInfo.StatusCode)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel() // nach 10 Sekunden abbrechen

		// not interessted in actual result, since all information is already written to the struct (ptr)
		_, err := m.Collection.InsertOne(ctx, metaData)
		if err != nil {
			return helpers.WrapError(err, helpers.FuncName()) // primitive.NilObjectID.Hex() ? probly useless
		}

		return nil
	}

}

// GetMataData returns the correct URLs based on moderation status
// to be embedded in a profile
func (m UploadModel) GetMetaData(profileOID primitive.ObjectID, executiveUserID string) ([]FileInfo, error) {

	var err error
	var data UploadHeader

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	err = m.Collection.FindOne(ctx, bson.M{"profileID": profileOID}).Decode(&data)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apperror.ErrNoData
		}
		// pass any other error
		return nil, helpers.WrapError(err, helpers.FuncName())
	}

	var fileInfo FileInfo
	var fileInfos []FileInfo

	// if moderation is enabled, return approved content only
	if os.Getenv("UPLOAD_MODERATION") == "YES" {
		// ToDo: check Admin Role - injection problem :-/
		executiveUserOID := helpers.ObjectID(executiveUserID)
		/*
			cred := m.GetCredentials(executiveUserID, false)
			fmt.Println(cred)
			if cred != nil {
				return nil, err
			}
		*/
		for _, s := range data.Slots {
			// creators see their pending content
			if s.Staged != nil {
				//if (s.Staged.UploadedID == executiveUserOID) || (cred.RoleCode == lookups.UserRoleAdmin) {
				if s.Staged.UploadedID == executiveUserOID {
					fileInfo.Description = s.Staged.Description
					fileInfo.StatusCode = s.Staged.StatusCode
					fileInfo.StatusText = database.GetLookupText(lookups.LookupType(lookups.LTcommentStatus), s.Staged.StatusCode)
					fileInfo.URL = s.Staged.SysFileName
					fileInfos = append(fileInfos, fileInfo)
				}
			} else {
				if s.Active != nil {
					fileInfo.Description = s.Active.Description
					fileInfo.StatusCode = s.Active.StatusCode
					fileInfo.StatusText = database.GetLookupText(lookups.LookupType(lookups.LTcommentStatus), s.Active.StatusCode)
					fileInfo.URL = s.Active.SysFileName
					fileInfos = append(fileInfos, fileInfo)
				}
			}
		}
	} else {
		for _, s := range data.Slots {
			if s.Active != nil {
				fileInfo.Description = s.Active.Description
				fileInfo.StatusCode = s.Active.StatusCode
				fileInfo.StatusText = database.GetLookupText(lookups.LookupType(lookups.LTcommentStatus), s.Active.StatusCode)
				fileInfo.URL = s.Active.SysFileName
				fileInfos = append(fileInfos, fileInfo)
			}
		}
	}

	return fileInfos, nil
}

// ToDO: DeleteMetaData

// since the upsert operation can not be used here, this function checks if there's already a document
// containing upload metadata for a profile
func (m UploadModel) uploadsExists(profileID primitive.ObjectID) (bool, error) {
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
	err := m.Collection.FindOne(ctx, bson.M{"profileID": profileID}, options.FindOne().SetProjection(fields)).Decode(&data)
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

// Generic Functions
/*
// SaveMetaData
// Logik im Controller (ob altes löschen etc.) hier nur save & delete
func (m UploadModel) SaveMetaData(uploadInfo *UploadInfo) error {

	// complete data
	uploadInfo.UploadedName, _ = m.GetUserNameOID(uploadInfo.UploadedID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	// since a custom ID is set by the controller, don't receivbe the result
	_, err := m.Collection.InsertOne(ctx, uploadInfo)
	if err != nil {
		return helpers.WrapError(err, helpers.FuncName()) // primitive.NilObjectID.Hex() ? probly useless
	}

	return nil
}

// ToDo: del(profileID)
*/
