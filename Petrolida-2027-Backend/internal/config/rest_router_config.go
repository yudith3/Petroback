package config

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/middleware"
	mylog "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/logger"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func NewRouter(server *gin.Engine, mw middleware.Middleware) *gin.Engine {
	//server.NoRoute(mw.HandleNoRoute())

	server.MaxMultipartMemory = 128 * 1024 * 1024
	server.Use(customRecovery())
	server.Use(mw.RequestID())
	// server.Use(mw.AbuseBanGuard())
	server.Use(mw.CORSMiddleware())
	server.Use(mw.BodyLimit(200 * 1024 * 1024)) // 200MB limit

	server.GET("/api/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	server.Static("/api/static", "./assets")
	return server
}

func customRecovery() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				var wrappedErr error
				if e, ok := err.(error); ok {
					wrappedErr = e
				} else {
					wrappedErr = fmt.Errorf("%v", err)
				}

				fmt.Println(mylog.ColorizePanic(fmt.Sprintf("\n[Recovery] Panic occurred: %v\n", err)))
				stack := debug.Stack()
				coloredStack := mylog.ColorizePanic(string(stack))

				fmt.Fprintln(os.Stderr, coloredStack)
				response.BuildResponseFailed("server panic occured", wrappedErr).
					SendWithAbort(ctx)
			}
		}()

		ctx.Next()
	}
}
