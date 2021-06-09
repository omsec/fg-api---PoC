package controllers

// Created is the standard response for new items
type Created struct {
	ID string `json:"id"`
}

// Uploaded is the standard response for new uploads
type Uploaded struct {
	URL        string `json:"url"`
	StatusCode int32  `json:"statusCode"`
	StatusText string `json:"statusText"`
}
