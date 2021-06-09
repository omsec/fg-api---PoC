package helpers

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ObjectID converts a string to a MongoDB ObjectID without the need of error checking
// (placed here so the database package is not required by the controllers package)
func ObjectID(ID string) primitive.ObjectID {
	id, err := primitive.ObjectIDFromHex(ID)
	if err != nil {
		return primitive.NilObjectID
	}
	return id
}

/*
// DocumentExists is used whenever an upsert operation is not applicable
// (eg. if a nested array must be handled)
// the collection must be passed via the models env injection to keep configuration centralized
func DocumentExists(collection *mongo.Collection, id primitive.ObjectID) (bool, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	// there seems to be no function like "exists" so a projection on just the ID is used
	fields := bson.D{
		{Key: "_id", Value: 1}}

	data := struct {
		ID primitive.ObjectID `bson:"_id"`
	}{}

	// some (old) sources say FindOne is slow and we should use find instead... (?)
	err := collection.FindOne(ctx, bson.M{"_id": id}, options.FindOne().SetProjection(fields)).Decode(&data)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
		// treat errors as a "yes" - caller should not evaluate the result in case of an error
		return true, WrapError(err, FuncName())
	}
	// no error means a document was found, hence the user does exist
	return true, nil
}
*/
