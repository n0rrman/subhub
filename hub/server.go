package main

import (
	"github.com/labstack/echo/v4"
)

func main() {
	// Initiate echo and the hub instance
	e := echo.New()
	h := hub{hubStore{}}
	h.store.init()

	// Available routes
	e.POST("/", h.handleSubscriber)
	e.GET("/publish", h.dummyPublisher)
	e.GET("/*", h.invalidRoute)

	// Listens on port 80
	e.Logger.Fatal(e.Start(":80"))
}
