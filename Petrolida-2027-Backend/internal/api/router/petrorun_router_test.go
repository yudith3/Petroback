package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/handler"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/service"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/gin-gonic/gin"
)

type petroRunScopeAuth struct{ service.AuthService }

func (petroRunScopeAuth) Authenticate(string) (domain.AdminProfile, error) {
	return domain.AdminProfile{Role: "division_admin"}, nil
}

func TestPetroRunAdminRoutesDenyOprecDivisionAdmins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	cfg := config.Config{}
	cfg.Auth.CookieName = "session"
	PetroRun(engine, handler.NewPetroRun(cfg.PetroRun, nil), petroRunScopeAuth{}, cfg)
	for _, route := range []struct{ method, path string }{{"GET", "/admin/orders"}, {"GET", "/admin/orders/summary"}, {"GET", "/admin/orders/PR27-000001"}, {"GET", "/admin/payment-proofs"}, {"GET", "/admin/payment-proofs/123/file"}, {"POST", "/admin/payment-proofs/123/review"}} {
		for _, authenticated := range []bool{false, true} {
			req := httptest.NewRequest(route.method, "/api/v1/petrorun"+route.path, nil)
			if authenticated {
				req.Header.Set("Authorization", "Bearer oprec-admin")
			}
			res := httptest.NewRecorder()
			engine.ServeHTTP(res, req)
			want := http.StatusUnauthorized
			if authenticated {
				want = http.StatusForbidden
			}
			if res.Code != want {
				t.Fatalf("%s %s authenticated=%v: got %d %s", route.method, route.path, authenticated, res.Code, res.Body.String())
			}
		}
	}
}
