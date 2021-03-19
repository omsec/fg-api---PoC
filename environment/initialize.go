package environment

import (
	"forza-garage/analytics"
	"forza-garage/database"
	"forza-garage/models"
	"os"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/mongo"
)

// Environment is used for dependency-injection (package de-coupling)
type Environment struct {
	Tracker     *analytics.Tracker
	UserModel   models.UserModel
	CourseModel models.CourseModel
}

// newEnv operates as the constructor to initialize the collection references (private)
func newEnv(mongoClient *mongo.Client, redisClient *redis.Client) *Environment {
	env := &Environment{}

	// prepare analytics gathering (profile visits)
	// always create the object so no futher checking is needed in the models
	env.Tracker = new(analytics.Tracker)
	env.Tracker.SetConnections(redisClient,
		mongoClient.Database(os.Getenv("DB_NAME")).Collection("analytics"))

	env.UserModel.Client = mongoClient
	env.UserModel.Collection = mongoClient.Database(os.Getenv("DB_NAME")).Collection("users") // ToDO: Const
	env.UserModel.Social = mongoClient.Database(os.Getenv("DB_NAME")).Collection("social")    // ToDO: Const

	// inject user model function to analytics tracker after its initialization
	env.Tracker.GetUserName = env.UserModel.GetUserName

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
	Env = newEnv(database.GetConnection(), database.GetRedisConnection())
}
