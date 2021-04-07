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
	"time"

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

// ToDO: move this function into its own file
func handleRequests() {
	router.Use(middleware.CORSMiddleware())

	// ToDo: Groups ?

	router.GET("/test", controllers.Test)

	router.GET("/lookups", controllers.ListLookups)

	// auth-related
	router.POST("/login", controllers.Login)
	router.POST("/logout", authentication.TokenAuthMiddleware(), controllers.Logout) // DELETE in Vorlage (umstritten)
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

	// analytics -- ToDo: nested URL, set obj type via controller GetVisitsCourse
	router.GET("/stats/visits", controllers.GetVisits)
	router.GET("/stats/visitors", authentication.TokenAuthMiddleware(), controllers.ListVisitors)

	// course
	// GET hat keinen BODY (Go/Gin & Postman unterstützen das zwar, Angular nicht) - deshalb Parameter
	// https://xspdf.com/resolution/58530870.html
	router.GET("/courses/public", controllers.ListCoursesPublic)
	router.GET("/courses/member", authentication.TokenAuthMiddleware(), controllers.ListCoursesMember)
	router.GET("/courses/public/:id", controllers.GetCoursePublic)
	router.GET("/courses/member/:id", authentication.TokenAuthMiddleware(), controllers.GetCourseMember)
	router.POST("/courses", authentication.TokenAuthMiddleware(), controllers.AddCourse)
	router.PUT("/courses/:id", authentication.TokenAuthMiddleware(), controllers.UpdateCourse)
	// ToDO: Delete
	// voting
	// for the sake of security I've created a public and a private endpoint, where the user is read from
	// the token, which is not present/necessary in the open one; again, they're handled by the same model
	// function, but different controllers
	router.GET("/courses/public/:id/votes", controllers.GetVotesPublic)
	router.GET("/courses/member/:id/votes", authentication.TokenAuthMiddleware(), controllers.GetVotesMember)
	router.POST("/course/vote", authentication.TokenAuthMiddleware(), controllers.CastVoteCourse)

	// logics
	router.POST("/course/exists", authentication.TokenAuthMiddleware(), controllers.ExistsForzaShare) // protected to prevent sniffs ;-)

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

	// connect to Analysis-DB (influxDB)
	if os.Getenv("USE_ANALYTICS") == "YES" {
		err = database.OpenInfluxConnection()
		if err != nil {
			log.Fatal(err)
		}
		defer database.CloseInfluxConnection()
	}

	// Inject DB-Connections to models
	environment.InitializeModels()

	// we're keeping track of client requests to control certain endpoints
	// hence we need to frequently shrink the list of recent requests
	requestTicker := time.NewTicker(time.Duration(1 * time.Minute)) // 5 * time.Second
	done := make(chan bool, 1)                                      // done channel can be shared, it's only used to stop the listener (select-loop)

	go func() {
		for {
			select {
			case <-done:
				return
			//case t := <-ticker.C:
			case <-requestTicker.C:
				environment.Env.Requests.Flush()
			}
		}
	}()

	// replicate profile visit log from cache to db
	/*
		replMins, err := strconv.Atoi(os.Getenv("ANALYTICS_REPLICATION_MINUTES"))
		if err != nil {
			log.Fatal("Invalid Configuration for env-Value ANALYTICS_REPLICATION_MINUTES")
		}
		ticker := time.NewTicker(time.Duration(replMins) * time.Minute)
		done := make(chan bool, 1)

		if os.Getenv("USE_ANALYTICS") == "YES" {
			go func() {
				for {
					select {
					case <-done:
						return
					//case t := <-ticker.C:
					case <-ticker.C:
						environment.Env.Tracker.Replicate()
					}
				}
			}()
		}
	*/

	fmt.Println("Forza-Garage running...")
	handleRequests()

	// save pending analytics to influxDB
	// placed here, because defer causes NIL panic
	environment.Env.Tracker.VisitorAPI.WriteAPI.Flush()
	environment.Env.Tracker.SearchAPI.WriteAPI.Flush()

	requestTicker.Stop()
	done <- true

}
