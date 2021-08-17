package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Sampler interface {
	GetSample() *ReviewItem
}

type Moderation struct {
	collections map[string]*mongo.Collection
}

// ReviewItem represent user content to review
// sent only - not directly saved
type ReviewItem struct {
	ParentID        *primitive.ObjectID // profile, comment...
	ParentType      *string
	ContentID       primitive.ObjectID // profile, upload, comment/reply...
	ContentType     string
	StatusCode      int32
	StatusText      string
	CreatorID       primitive.ObjectID
	CreatorName     string
	CreatorJoinedAt time.Time // have client display "member since...
	// ToDO: überlegen, how many...
	// CreatorPosts int
	// CreatorReports int
	// PostReports int
}

// SetConnections initializes the instance
func (m *Moderation) SetConnections(mongoCollections map[string]*mongo.Collection) {
	m.collections = mongoCollections
}

// GetModerationItem randomly chooses some pending or reported content for review
func (m *Moderation) GetModerationItem() *ReviewItem {

	// get samples from every collection
	// https://docs.mongodb.com/manual/reference/operator/aggregation-pipeline/#aggregation-pipeline-operator-reference

	// use injected functions since collections have different formats, uknown to the moderation type

	return nil
}
