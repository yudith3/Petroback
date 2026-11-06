package router

import (
	"context"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/handler"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/service"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type publicAnnouncementRepo struct{}

func (publicAnnouncementRepo) GetInterviewResult(_ context.Context, nrp string) (entity.InterviewResult, error) {
	switch nrp {
	case "5070261234":
		return entity.InterviewResult{NRP: nrp, Name: "Published Applicant", Status: "accepted", Subdivision: &entity.Subdivisi{ID: uuid.New(), Nama: "Web Development"}}, nil
	case "351518190408001":
		return entity.InterviewResult{NRP: nrp, Status: "accepted"}, nil
	case "5019261031":
		return entity.InterviewResult{NRP: nrp, Status: "on_hold"}, nil
	default:
		return entity.InterviewResult{}, domain.ErrRegistrantNotFound
	}
}

func (publicAnnouncementRepo) GetByNRP(_ context.Context, nrp string) (entity.OprecStaff, error) {
	if nrp != "5070261234" && nrp != "5070261235" {
		return entity.OprecStaff{}, domain.ErrRegistrantNotFound
	}
	status := entity.OprecStaffStatusAccepted
	if nrp == "5070261235" {
		status = entity.OprecStaffStatusRejected
	}
	return entity.OprecStaff{Nama: "Test Applicant", NRP: nrp, Email: "private@example.com", NomorTelepon: "08123456789", TentangPetrolida: "private answer", Status: status, AcceptedDivisi: &entity.Subdivisi{ID: uuid.New(), Nama: "Web Development"}}, nil
}

func TestPublishedInterviewRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	svc := service.NewAnnouncement(publicAnnouncementRepo{}, config.OprecConfig{InterviewAnnouncementAt: time.Now().Add(-time.Hour)})
	Announcements(engine, handler.NewAnnouncement(svc))
	for _, tc := range []struct {
		nrp    string
		status int
		code   string
	}{
		{"5070261234", 200, "accepted"},
		{"351518190408001", 200, "accepted"},
		{"5019261031", 409, "ANNOUNCEMENT_ON_HOLD"},
		{"5070261235", 200, "rejected"},
		{"5070261236", 404, "REGISTRANT_NOT_FOUND"},
		{"351518190408002", 400, "VALIDATION_ERROR"},
	} {
		t.Run(tc.nrp, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/announcements/interview", strings.NewReader(`{"nrp":"`+tc.nrp+`"}`))
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()
			engine.ServeHTTP(res, req)
			if res.Code != tc.status || !strings.Contains(res.Body.String(), tc.code) {
				t.Fatalf("response = %d %s", res.Code, res.Body.String())
			}
			if res.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("missing cache control")
			}
			for _, secret := range []string{"private@example.com", "08123456789", "private answer", "documents"} {
				if strings.Contains(res.Body.String(), secret) {
					t.Fatalf("public response leaked %s", secret)
				}
			}
		})
	}
}
func TestPublicAnnouncementRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	svc := service.NewAnnouncement(publicAnnouncementRepo{}, config.OprecConfig{AnnouncementAt: time.Now().Add(-time.Hour)})
	Announcements(engine, handler.NewAnnouncement(svc))
	cases := []struct {
		method, path, body string
		status             int
		code               string
	}{
		{"GET", "/api/v1/announcements/config", "", 200, "server_time"},
		{"POST", "/api/v1/announcements/staff", `{"nrp":"5070261234"}`, 200, "accepted"},
		{"POST", "/api/v1/announcements/staff", `{"nrp":"5070261235"}`, 200, "rejected"},
		{"POST", "/api/v1/announcements/staff", `{"nrp":"5070261236"}`, 404, "REGISTRANT_NOT_FOUND"},
		{"POST", "/api/v1/announcements/interview", `{"nrp":"5070261234"}`, 403, "ANNOUNCEMENT_CLOSED"},
		{"POST", "/api/v1/announcements/staff", `{"nrp":"invalid"}`, 400, "VALIDATION_ERROR"},
		{"POST", "/api/v1/announcements/staff", `{`, 400, "VALIDATION_ERROR"},
		{"POST", "/api/v1/announcements/unknown", `{"nrp":"5070261234"}`, 404, "ROUTE_NOT_FOUND"},
	}
	for _, tc := range cases {
		t.Run(tc.path+tc.body, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()
			engine.ServeHTTP(res, req)
			if res.Code != tc.status || !strings.Contains(res.Body.String(), tc.code) {
				t.Fatalf("response = %d %s", res.Code, res.Body.String())
			}
			if res.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("missing cache control")
			}
			for _, secret := range []string{"private@example.com", "08123456789", "private answer", "documents"} {
				if strings.Contains(res.Body.String(), secret) {
					t.Fatalf("public response leaked %s", secret)
				}
			}
		})
	}
}
