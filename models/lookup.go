package models

// Lookup is nested in models to reference a code
// don't confuse with the structures of the lookup-package, which deals with the code types
// sent to the client as a map
type Lookup struct {
	Value int32 `json:"value" bson:"value"`
	// nur die angefragte Sprache liefern (User/Header)
	Text string `json:"text" bson:"-"`
	//TextDE string `json:"textDE" bson:"-"`
}
