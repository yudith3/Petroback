package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"github.com/gin-gonic/gin"
)

func TestPetroRunConfigDoesNotChangeOrderSurcharges(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewPetroRun(config.PetroRunConfig{
		BasePriceIDR:      150_000,
		JerseySurcharges:  map[string]int64{"2XL": 5_000, "3XL": 10_000, "4XL": 15_000},
		BankName:          "BNI",
		BankAccountNumber: "1234567890",
		BankAccountHolder: "Petrolida ITS",
		AccessSecret:      "test-secret-at-least-32-characters-long",
	}, nil)
	want := map[string]int64{"S": 0, "M": 0, "L": 0, "XL": 0, "2XL": 5_000, "3XL": 10_000, "4XL": 15_000}

	// Dipanggil dua kali: endpoint config tidak boleh mengubah tabel ukuran
	// yang juga dipakai untuk menghitung harga order.
	for range 2 {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/petrorun/config", nil)
		h.Config(ctx)

		var body struct {
			Data struct {
				AvailableSizes []config.SizeConfig `json:"available_sizes"`
			} `json:"data"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		for _, size := range body.Data.AvailableSizes {
			if size.SurchargeIDR != want[size.Code] {
				t.Fatalf("response %s surcharge = %d, want %d", size.Code, size.SurchargeIDR, want[size.Code])
			}
		}
	}
	for code, surcharge := range want {
		if size, _ := h.cfg.JerseySize(code); size.SurchargeIDR != surcharge {
			t.Fatalf("order pricing %s surcharge = %d, want %d", code, size.SurchargeIDR, surcharge)
		}
	}
}
