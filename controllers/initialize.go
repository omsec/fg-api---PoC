package controllers

import (
	"forza-garage/database"
	"forza-garage/models"
	"os"

	"go.mongodb.org/mongo-driver/mongo"
)

// Env is used for dependency-injection (package de-coupling)
type Env struct {
	userModel   models.UserModel
	courseModel models.CourseModel
}

// newEnv operates as the constructor to initialize the collection references (private)
func newEnv(client *mongo.Client) *Env {
	env := &Env{}

	env.userModel.Client = client
	env.userModel.Collection = client.Database(os.Getenv("DB_NAME")).Collection("users") // ToDO: Const
	env.userModel.Social = client.Database(os.Getenv("DB_NAME")).Collection("social")    // ToDO: Const

	env.courseModel.Client = client
	env.courseModel.Collection = client.Database(os.Getenv("DB_NAME")).Collection("racing") // ToDO: Const

	return env
}

// singleton registry
var env *Env

// Initialize injects the database connection to the models
// (do not confuse with package init)
func Initialize() {
	/*env = &Env{
	userModel: models.UserModel{Client: database.GetConnection()}}*/
	env = newEnv(database.GetConnection())
}
