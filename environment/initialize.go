package environment

import (
	"forza-garage/database"
	"forza-garage/models"
	"os"

	"go.mongodb.org/mongo-driver/mongo"
)

// Environment is used for dependency-injection (package de-coupling)
type Environment struct {
	UserModel   models.UserModel
	CourseModel models.CourseModel
}

// newEnv operates as the constructor to initialize the collection references (private)
func newEnv(client *mongo.Client) *Environment {
	env := &Environment{}

	env.UserModel.Client = client
	env.UserModel.Collection = client.Database(os.Getenv("DB_NAME")).Collection("users") // ToDO: Const
	env.UserModel.Social = client.Database(os.Getenv("DB_NAME")).Collection("social")    // ToDO: Const

	env.CourseModel.Client = client
	env.CourseModel.Collection = client.Database(os.Getenv("DB_NAME")).Collection("racing") // ToDO: Const
	// Funktionen aus dem User Model in's Course model "injecten"
	env.CourseModel.GetUserName = env.UserModel.GetUserName
	env.CourseModel.CredentialsReader = env.UserModel.GetCredentials

	return env
}

// Env is the singleton registry
var Env *Environment

// InitializeModels injects the database connection to the models
// (do not confuse with package init)
func InitializeModels() {
	/*env = &Env{
	userModel: models.UserModel{Client: database.GetConnection()}}*/
	Env = newEnv(database.GetConnection())
}
