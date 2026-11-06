package router

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/handler"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/service"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/export"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

type exportAuthService struct {
	service.AuthService
}

func (exportAuthService) Authenticate(token string) (domain.AdminProfile, error) {
	if token == "superadmin-token" {
		return domain.AdminProfile{Role: string(entity.RoleSuperAdmin)}, nil
	}
	return domain.AdminProfile{}, domain.ErrUnauthenticated
}

type exportOprecService struct {
	service.OprecStaffService
	gotStatus string
}

func (s *exportOprecService) Export(_ domain.AdminProfile, params domain.RegistrantListParams) (export.Table, error) {
	s.gotStatus = params.Status
	return export.Table{
		Columns: []export.Column{{Header: "Nama"}, {Header: "NRP", Text: true}},
		Rows:    [][]string{{"=HYPERLINK(\"http://x\")", "081234567890"}},
	}, nil
}

func TestOprecStaffExportCSVRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oprecService := &exportOprecService{}
	engine := gin.New()
	cfg := config.Config{Auth: config.AuthConfig{CookieName: "petrolida_admin_session"}}
	OprecStaff(engine, handler.NewOprecStaff(oprecService), nil, exportAuthService{}, cfg)

	cases := []struct {
		name   string
		token  string
		query  string
		status int
	}{
		{name: "anonymous", status: http.StatusUnauthorized},
		{name: "invalid token", token: "participant-token", status: http.StatusUnauthorized},
		{name: "invalid filter", token: "superadmin-token", query: "?status=unknown", status: http.StatusUnprocessableEntity},
		{name: "superadmin", token: "superadmin-token", query: "?status=interview", status: http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/oprec-staff/export/csv"+tc.query, nil)
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			res := httptest.NewRecorder()
			engine.ServeHTTP(res, req)
			if res.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", res.Code, tc.status, res.Body.String())
			}
			if tc.status != http.StatusOK {
				if strings.Contains(res.Body.String(), "081234567890") {
					t.Fatal("rejected request leaked export data")
				}
				return
			}
			if got := res.Header().Get("Content-Type"); got != "text/csv; charset=utf-8" {
				t.Errorf("Content-Type = %q", got)
			}
			disposition := res.Header().Get("Content-Disposition")
			if !strings.HasPrefix(disposition, `attachment; filename="oprec-staff-`) || !strings.HasSuffix(disposition, `.csv"`) {
				t.Errorf("Content-Disposition = %q", disposition)
			}
			if got := res.Header().Get("Cache-Control"); got != "private, no-store" {
				t.Errorf("Cache-Control = %q", got)
			}
			if oprecService.gotStatus != "interview" {
				t.Errorf("service status filter = %q, want interview", oprecService.gotStatus)
			}
			body := res.Body.String()
			for _, want := range []string{"\xEF\xBB\xBFNama,NRP\n", `"'=HYPERLINK(""http://x"")"`, `"=""081234567890"""`} {
				if !strings.Contains(body, want) {
					t.Errorf("body does not contain %q: %q", want, body)
				}
			}
		})
	}
}

func TestOprecStaffExportExcelRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	cfg := config.Config{Auth: config.AuthConfig{CookieName: "petrolida_admin_session"}}
	OprecStaff(engine, handler.NewOprecStaff(&exportOprecService{}), nil, exportAuthService{}, cfg)

	anonymous := httptest.NewRecorder()
	engine.ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, "/api/v1/oprec-staff/export/excel", nil))
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d, want 401", anonymous.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/oprec-staff/export/excel", nil)
	req.AddCookie(&http.Cookie{Name: "petrolida_admin_session", Value: "superadmin-token"})
	res := httptest.NewRecorder()
	engine.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", res.Code, res.Body.String())
	}
	if got, want := res.Header().Get("Content-Type"), "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}
	if disposition := res.Header().Get("Content-Disposition"); !strings.HasSuffix(disposition, `.xlsx"`) {
		t.Errorf("Content-Disposition = %q", disposition)
	}

	file, err := excelize.OpenReader(res.Body)
	if err != nil {
		t.Fatalf("open XLSX: %v", err)
	}
	defer file.Close()
	rows, err := file.GetRows(file.GetSheetName(0))
	if err != nil {
		t.Fatalf("GetRows() error = %v", err)
	}
	want := [][]string{{"Nama", "NRP"}, {`'=HYPERLINK("http://x")`, "081234567890"}}
	if len(rows) != len(want) || !slices.Equal(rows[0], want[0]) || !slices.Equal(rows[1], want[1]) {
		t.Fatalf("rows = %q, want %q", rows, want)
	}
}
