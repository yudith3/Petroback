package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/gin-gonic/gin"
)

func TestRequireSuperAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		profile    domain.AdminProfile
		setProfile bool
		wantStatus int
	}{
		{
			name: "superadmin allowed", profile: domain.AdminProfile{Role: string(entity.RoleSuperAdmin)},
			setProfile: true, wantStatus: http.StatusNoContent,
		},
		{
			name: "division admin denied", profile: domain.AdminProfile{Role: string(entity.RoleDivisionAdmin)},
			setProfile: true, wantStatus: http.StatusForbidden,
		},
		{name: "missing profile denied", wantStatus: http.StatusForbidden},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			if test.setProfile {
				router.Use(func(ctx *gin.Context) {
					ctx.Set(AdminProfileContextKey, test.profile)
					ctx.Next()
				})
			}
			router.GET("/protected", RequireSuperAdmin(), func(ctx *gin.Context) {
				ctx.Status(http.StatusNoContent)
			})

			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/protected", nil))
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
		})
	}
}
