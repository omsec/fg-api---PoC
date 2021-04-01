package environment

import (
	"forza-garage/analytics"
	"forza-garage/database"
	"forza-garage/models"
	"os"

	influxdb2 "github.com/influxdata/influxdb-client-go"
	"go.mongodb.org/mongo-driver/mongo"
)

// Environment is used for dependency-injection (package de-coupling)
type Environment struct {
	Tracker     *analytics.Tracker
	UserModel   models.UserModel
	CourseModel models.CourseModel
}

// newEnv operates as the constructor to initialize the collection references (private)
func newEnv(mongoClient *mongo.Client, influxClient *influxdb2.Client) *Environment {
	env := &Environment{}

	// ToDO: mongoClient für Modelle entfernen

	// prepare analytics gathering (profile visits)
	// always create the object so no futher checking is needed in the models
	env.Tracker = new(analytics.Tracker)
	env.Tracker.SetConnections(
		influxClient, // brauchts nicht mehr hier
		mongoClient.Database(os.Getenv("DB_NAME")).Collection("analytics"))
	// weil pointer umweg über varIABLE
	fluxClient := *influxClient
	env.Tracker.VisitorAPI.WriteAPI = fluxClient.WriteAPIBlocking(os.Getenv("ANALYTICS_ORG"), os.Getenv("ANALYTICS_VISITORS_BUCKET"))
	env.Tracker.VisitorAPI.QueryAPI = fluxClient.QueryAPI(os.Getenv("ANALYTICS_ORG"))
	env.Tracker.VisitorAPI.DeleteAPI = fluxClient.DeleteAPI()
	env.Tracker.SearchAPI.WriteAPI = fluxClient.WriteAPIBlocking(os.Getenv("ANALYTICS_ORG"), os.Getenv("ANALYTICS_SEARCHES_BUCKET"))
	env.Tracker.SearchAPI.QueryAPI = fluxClient.QueryAPI(os.Getenv("ANALYTICS_ORG"))
	// no deletes required for search bucket (TTL set)

	env.UserModel.Client = mongoClient
	env.UserModel.Collection = mongoClient.Database(os.Getenv("DB_NAME")).Collection("users") // ToDO: Const
	env.UserModel.Social = mongoClient.Database(os.Getenv("DB_NAME")).Collection("social")    // ToDO: Const

	// inject user model function to analytics tracker after its initialization
	env.Tracker.GetUserName = env.UserModel.GetUserName
	env.Tracker.GetUserNameOID = env.UserModel.GetUserNameOID

	env.CourseModel.Client = mongoClient
	env.CourseModel.Collection = mongoClient.Database(os.Getenv("DB_NAME")).Collection("racing") // ToDO: Const
	// Funktionen aus dem User Model in's Course model "injecten"
	env.CourseModel.GetUserName = env.UserModel.GetUserName
	env.CourseModel.CredentialsReader = env.UserModel.GetCredentials
	// inject analytics
	env.CourseModel.Tracker = env.Tracker

	return env
}

// Env is the singleton registry
var Env *Environment

// InitializeModels injects the database connection to the models
// (do not confuse with package init)
func InitializeModels() {
	/*env = &Env{
	userModel: models.UserModel{Client: database.GetConnection()}}*/

	Env = newEnv(database.GetConnection(), database.GetInfluxConnection())
}
