package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestChoicePriorityListAndExportParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	id := uuid.New()
	for _, test := range []struct {
		query    string
		priority int16
		invalid  bool
	}{
		{"?division_id=" + id.String() + "&choice_priority=1", 1, false},
		{"?subdivision_id=" + id.String() + "&choice_priority=2", 2, false},
		{"?division_id=" + id.String() + "&choice_priority=all", 0, false},
		{"?division_id=" + id.String(), 0, false},
		{"?division_id=bad&choice_priority=1", 0, true},
		{"?division_id=" + id.String() + "&choice_priority=3", 0, true},
		{"?choice_priority=1", 0, true},
	} {
		t.Run(test.query, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("GET", "/api/v1/oprec-staff"+test.query, nil)
			params, err := parseListParams(ctx)
			if (err != nil) != test.invalid {
				t.Fatalf("error = %v", err)
			}
			if err == nil && params.ChoicePriority != test.priority {
				t.Fatalf("priority = %d, want %d", params.ChoicePriority, test.priority)
			}
		})
	}
}
