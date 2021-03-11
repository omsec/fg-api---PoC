package main

import (
	"fmt"
	"forza-garage/authentication"
	"forza-garage/controllers"
	"forza-garage/database"
	"forza-garage/environment"
	"forza-garage/middleware"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

var (
	router = gin.Default()
)

// wird VOR der Programmausführung (main) gerufen
// die Reihenfolge der Package-Inits ist aber undefiniert!
func init() {
	// Load Config
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

func handleRequests() {
	router.Use(middleware.CORSMiddleware())

	// ToDo: Groups ?

	router.GET("/test", controllers.Test)

	router.GET("/lookups", controllers.ListLookups)

	// auth-related
	router.POST("/login", controllers.Login)
	router.POST("/logout", authentication.TokenAuthMiddleware(), controllers.Logout) // DELETE in Vorlage (Achtung Server-Rechte)
	router.POST("/refresh", controllers.Refresh)                                     // nicht prüfen, ob das at noch valide ist (keine Middleware)
	router.POST("/register", controllers.Register)

	router.POST("/user/exists", controllers.UserExists)
	router.POST("/email/exists", controllers.EMailExists)

	// user-mgmt
	router.GET("/users/:id", authentication.TokenAuthMiddleware(), controllers.GetUser)
	router.POST("/user/changePass", authentication.TokenAuthMiddleware(), controllers.ChangePassword)
	router.POST("/user/verifyPass", authentication.TokenAuthMiddleware(), controllers.VerifyPassword)

	router.POST("/users/:id/blocked", authentication.TokenAuthMiddleware(), controllers.BlockUser)
	router.DELETE("/users/:id/blocked", authentication.TokenAuthMiddleware(), controllers.UnblockUser)

	router.GET("/users/:id/friends", authentication.TokenAuthMiddleware(), controllers.GetFriends)
	router.POST("/users/:id/friends", authentication.TokenAuthMiddleware(), controllers.AddFriend)
	router.DELETE("/users/:id/friends", authentication.TokenAuthMiddleware(), controllers.RemoveFriend) // ToDo: anpassn {id}

	router.GET("/users/:id/followings", authentication.TokenAuthMiddleware(), controllers.GetFollowings)
	router.POST("/users/:id/followings", authentication.TokenAuthMiddleware(), controllers.FollowUser) // ToDo: Vs Verb "follow"

	router.GET("/users/:id/followers", authentication.TokenAuthMiddleware(), controllers.GetFollowers)

	// course
	// GET hat keinen BODY (Go/Gin & Postman unterstützen das zwar, Angular nicht) - deshalb Parameter
	// https://xspdf.com/resolution/58530870.html
	router.GET("/courses", controllers.ListCourses)
	router.GET("/courses/:id", controllers.GetCourse)
	router.POST("/courses", authentication.TokenAuthMiddleware(), controllers.AddCourse)
	router.PUT("/courses/:id", authentication.TokenAuthMiddleware(), controllers.UpdateCourse)
	// ToDO: Delete

	router.POST("/course/exists", authentication.TokenAuthMiddleware(), controllers.ExistsForzaShare) // protected to prevent sniffs ;-)

	/*
		URL scheme:
		router.GET("/courses", controllers.ListCourses) 200 | 204
		router.GET("/courses/:id", controllers.GetCourse) 200 | 204
		router.POST("/courses", authentication.TokenAuthMiddleware(), controllers.AddCourse) 201 | 422 | (permission)
		router.PUT("/courses/:id", authentication.TokenAuthMiddleware(), controllers.UpdateCourse) 200 | 422
		router.DELETE("/courses/:id", authentication.TokenAuthMiddleware(), controllers.UpdateCourse) 200 | 422
		// logics (domain-singular/verb)
		router.POST("/courses/exists", authentication.TokenAuthMiddleware(), controllers.ExistsForzaShare) // protected to prevent sniffs ;-)
	*/

	switch os.Getenv("APP_ENV") {
	case "DEV":
		router.Run(":" + os.Getenv("API_PORT"))
	case "PRD":
		router.RunTLS(":"+os.Getenv("API_PORT"), os.Getenv("APP_CERTFILE"), os.Getenv("APP_KEYFILE"))
	default:
		panic(fmt.Errorf("APP_ENV must not set"))
	}

	// https://github.com/denji/golang-tls
	// router.RunTLS()
}

func main() {
	// Connect to main database here (mongoDB)
	err := database.OpenConnection()
	if err != nil {
		log.Fatal(err)
	}
	defer database.CloseConnection()

	// connect to JWT Store (redis)
	err = authentication.OpenConnection()
	if err != nil {
		log.Fatal(err)
	}
	defer authentication.CloseConnection()

	// connect to Analysis-DB (redis)
	err = database.OpenRedisConnection()
	if err != nil {
		log.Fatal(err)
	}
	defer database.CloseConnection()

	// Initialize the Models
	environment.Initialize()

	fmt.Println("Forza-Garage running...")
	handleRequests()
}
