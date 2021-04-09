package main

import (
	"fmt"
	"forza-garage/authentication"
	"forza-garage/database"
	"forza-garage/environment"
	"log"
	"os"
	"strconv"
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
	replMins, err := strconv.Atoi(os.Getenv("ANALYTICS_REPLICATION_MINUTES"))
	if err != nil {
		log.Fatal("Invalid Configuration for env-Value ANALYTICS_REPLICATION_MINUTES")
	}
	replTicker := time.NewTicker(time.Duration(replMins) * time.Minute)
	//replTicker := time.NewTicker(time.Duration(replMins) * 10 * time.Second)
	// ToDo: save TS of last replication into a control file
	// this ensures repl is run even if the server runs shirter than the interval

	if os.Getenv("USE_ANALYTICS") == "YES" {
		go func() {
			for {
				select {
				case <-done:
					return
				//case t := <-ticker.C:
				case <-replTicker.C:
					environment.Env.Tracker.Replicate()
				}
			}
		}()
	}

	fmt.Println("Forza-Garage running...")
	handleRequests()

	// ToDO: Wird das überhaupt aufgerufen? => NEIN
	// Muss das evtl. in einen SigTerm-Handler?
	fmt.Println("shutdown test")

	// save pending analytics to influxDB
	// placed here, because defer causes NIL panic
	environment.Env.Tracker.VisitorAPI.WriteAPI.Flush()
	environment.Env.Tracker.SearchAPI.WriteAPI.Flush()

	requestTicker.Stop()
	replTicker.Stop()
	done <- true

}
