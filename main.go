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
	router.POST("/user/changePass", authentication.TokenAuthMiddleware(), controllers.ChangePassword)

	// course
	router.POST("/course/add", authentication.TokenAuthMiddleware(), controllers.AddCourse)
	router.GET("/courses", controllers.ListCourses)

	/*
		URL scheme:
		router.POST("/aufgabe/:id", authentication.TokenAuthMiddleware(), controllers.GetAufgabe)
		router.POST("/aufgaben", authentication.TokenAuthMiddleware(), controllers.ListAufgaben)
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
