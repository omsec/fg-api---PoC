package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Social is an internal type, not sent to clients.
// It is used to pass some meta-information from the voting-system (castVote) to
// referenced parents (Course, Comments etc.) to store it there
// In a future refactoring, this could make up a Header-Type...
type Social = struct {
	ProfileOID primitive.ObjectID
	Rating     float32
	SortOrder  float32
	UpVotes    int32
	DownVotes  int32
	TouchedTS  time.Time // a vote updates the "touched" info, not the "modified"
}
