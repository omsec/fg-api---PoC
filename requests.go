package main

import (
	"fmt"
	"forza-garage/authentication"
	"forza-garage/controllers"
	"forza-garage/middleware"
	"os"
)

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

	// nicht öffentlich, kein aufruf für andere als der aktuelle user vorgesehen (daher kein param)
	router.POST("/user/blocked", authentication.TokenAuthMiddleware(), controllers.BlockUser)
	router.DELETE("/user/blocked", authentication.TokenAuthMiddleware(), controllers.UnblockUser)

	router.GET("/user/votes", authentication.TokenAuthMiddleware(), controllers.GetUserVotes) // nur noch für (eigenes) profil als übersicht
	// ToDo: /user/comments

	// öffentlich/einsehbar, aufruf auch für profile anderer user (daher mit param)
	router.GET("/users/:id/friends", authentication.TokenAuthMiddleware(), controllers.GetFriends)
	router.POST("/users/:id/friends", authentication.TokenAuthMiddleware(), controllers.AddFriend)
	router.DELETE("/users/:id/friends", authentication.TokenAuthMiddleware(), controllers.RemoveFriend) // ToDo: anpassn {id}

	router.GET("/users/:id/followings", authentication.TokenAuthMiddleware(), controllers.GetFollowings)
	router.POST("/users/:id/followings", authentication.TokenAuthMiddleware(), controllers.FollowUser) // ToDo: Vs Verb "follow"

	router.GET("/users/:id/followers", authentication.TokenAuthMiddleware(), controllers.GetFollowers)

	// system tools
	router.GET("/monitor/requests/count", authentication.TokenAuthMiddleware(), controllers.CountRequests)
	router.GET("/monitor/requests/dump", authentication.TokenAuthMiddleware(), controllers.DumpRequests)
	router.POST("/monitor/requests/flush", authentication.TokenAuthMiddleware(), controllers.FlushRequests)

	// analytics
	router.GET("/stats/visitors", authentication.TokenAuthMiddleware(), controllers.ListVisitors)

	// voting
	router.POST("/vote", authentication.TokenAuthMiddleware(), controllers.CastVote)

	// commenting
	router.POST("/comment", authentication.TokenAuthMiddleware(), controllers.AddComment) // easier handling for client

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
	// statistics
	router.GET("/courses/public/:id/visits", controllers.GetCourseVisits) // visits since last 7 days "hot"
	// commenting
	router.GET("/courses/public/:id/comments", controllers.ListCommentsPublic)
	router.GET("/courses/member/:id/comments", authentication.TokenAuthMiddleware(), controllers.ListCommentsMember)

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
