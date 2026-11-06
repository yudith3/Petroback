package handler

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPetroRunAdminDateFiltersAndAllowlist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, query := range []string{"sort=DROP+TABLE", "status=paid", "direction=invalid", "created_from=invalid", "created_from=2026-10-21&created_to=2026-10-20", "transaction_from=2026-10-21&transaction_to=2026-10-20"} {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest("GET", "/?"+query, nil)
		if _, err := parsePetroRunAdminQuery(ctx); err == nil {
			t.Fatalf("accepted invalid query %s", query)
		}
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/?created_from=2026-10-19&created_to=2026-10-19", nil)
	q, err := parsePetroRunAdminQuery(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if q.CreatedFrom.UTC().Format(time.RFC3339) != "2026-10-18T17:00:00Z" || q.CreatedTo.Sub(*q.CreatedFrom) != 24*time.Hour {
		t.Fatalf("incorrect WIB range %v %v", q.CreatedFrom, q.CreatedTo)
	}
}
