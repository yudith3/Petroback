package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/service"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type confirmationServiceStub struct{ service.OprecStaffService }

func (confirmationServiceStub) ConfirmRegistration(_ context.Context, key string) (domain.RegistrationConfirmation, error) {
	_, err := uuid.Parse(key)
	if err != nil {
		return domain.RegistrationConfirmation{}, myerror.NewFieldValidation("submission_key", "Submission", "invalid")
	}
	return domain.RegistrationConfirmation{State: "unconfirmed"}, nil
}
func TestConfirmationEndpointDoesNotCacheOrAcceptOversizedBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/status", NewOprecStaff(confirmationServiceStub{}).ConfirmRegistration)
	for _, body := range []string{`{"submission_key":"` + uuid.NewString() + `"}`, `{"submission_key":"` + strings.Repeat("a", 2000) + `"}`, `{broken`} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest("POST", "/status", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		if recorder.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("confirmation can be cached")
		}
		var value map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &value); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(body, "aaaa") || body == `{broken` {
			if recorder.Code != 422 {
				t.Fatalf("invalid body returned %d", recorder.Code)
			}
		} else if recorder.Code != 200 || value["status"] != true {
			t.Fatalf("confirmation = %d %s", recorder.Code, recorder.Body)
		}
	}
}
