package router

import (
	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/handler"
	"github.com/gin-gonic/gin"
)

func MasterData(app *gin.Engine, masterdatahandler handler.MasterDataHandler) {
	routes := app.Group("/api/v1/master")
	{
		routes.GET("/division", masterdatahandler.GetsDivisions)
		routes.GET("/division/sub", masterdatahandler.GetSubDivisions)
		routes.GET("/division/sub/:id", masterdatahandler.GetSubDivisionByID)
		routes.GET("/division/:id", masterdatahandler.GetDivisionByID)
		routes.GET("/department", masterdatahandler.GetDepartements)
		routes.GET("/department/:id", masterdatahandler.GetDepartmentByID)
		routes.GET("/faculty", masterdatahandler.GetFaculties)
		routes.GET("/faculty/:id", masterdatahandler.GetFacultyByID)
		routes.GET("/divisions", masterdatahandler.GetsDivisions)
		routes.GET("/subdivisions", masterdatahandler.GetSubDivisions)
		routes.GET("/departments", masterdatahandler.GetDepartements)
		routes.GET("/faculties", masterdatahandler.GetFaculties)
	}
}
