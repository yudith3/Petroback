package router

import (
	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/handler"
	"github.com/gin-gonic/gin"
)

func Announcements(engine *gin.Engine, h *handler.AnnouncementHandler) {
	group := engine.Group("/api/v1/announcements")
	group.GET("/config", h.Config)
	group.POST("/:stage", h.Lookup)
}
