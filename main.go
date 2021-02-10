package main

import (
	"fmt"
	"forza-garage/authentication"
	"forza-garage/controllers"
	"forza-garage/database"
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

	router.GET("/lookups", controllers.ListLookups)

	// auth-related
	router.POST("/login", controllers.Login)
	router.POST("/logout", authentication.TokenAuthMiddleware(), controllers.Logout) // DELETE in Vorlage (Achtung Server-Rechte)
	router.POST("/refresh", controllers.Refresh)                                     // nicht prüfen, ob das at noch valide ist (keine Middleware)
	router.POST("/user/exists", controllers.UserExists)
	router.POST("/register", controllers.Register)

	// user-mgmt
	router.GET("users/:id", authentication.TokenAuthMiddleware(), controllers.GetUser)
	router.POST("/user/changePass", authentication.TokenAuthMiddleware(), controllers.ChangePassword)
	router.POST("/user/verifyPass", authentication.TokenAuthMiddleware(), controllers.VerifyPassword)
	router.POST("/user/addFriend", authentication.TokenAuthMiddleware(), controllers.AddFriend)
	router.POST("/user/removeFriend", authentication.TokenAuthMiddleware(), controllers.RemoveFriend)

	// course
	// GET hat keinen BODY (Go/Gin & Postman unterstützen das zwar, Angular nicht) - deshalb Parameter
	// https://xspdf.com/resolution/58530870.html
	router.GET("/courses", controllers.ListCourses)
	router.GET("/courses/:id", controllers.GetCourse)
	router.POST("/course/add", authentication.TokenAuthMiddleware(), controllers.AddCourse)
	router.PUT("/course/edit/:id", authentication.TokenAuthMiddleware(), controllers.UpdateCourse)
	router.POST("/course/exists", authentication.TokenAuthMiddleware(), controllers.ExistsForzaShare) // protected to prevent sniffs ;-)

	/*
		URL scheme:
		router.GET("/aufgaben", authentication.TokenAuthMiddleware(), controllers.ListAufgaben)
		router.GET("/aufgabe/:id", authentication.TokenAuthMiddleware(), controllers.GetAufgabe)
		router.DELETE("/aufgabe/:id", authentication.TokenAuthMiddleware(), controllers.DeleteAufgabe)
		router.POST("/aufgaben/add", authentication.TokenAuthMiddleware(), controllers.CreateAufgabe)
		router.POST("/aufgaben/edit/:id", authentication.TokenAuthMiddleware(), controllers.UpdateAufgabe)
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

	// Initialize the Models
	controllers.Initialize()

	fmt.Println("Forza-Garage running...")
	handleRequests()
}
