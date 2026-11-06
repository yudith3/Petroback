package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
)

type PetroRunConfig struct {
	BasePriceIDR                                   int64
	AdminFeeIDR                                    int64
	JerseySurcharges                               map[string]int64
	BankName, BankAccountNumber, BankAccountHolder string
	// AccessSecret adalah kunci HMAC untuk token akses order (pengganti login
	// peserta). Mengganti nilainya membatalkan token semua order.
	AccessSecret string
	Sales        *domain.PetroRunSaleRules
	TestMode     bool
}

func (c PetroRunConfig) Available() bool {
	return c.BasePriceIDR > 0 && c.BankName != "" && c.BankAccountNumber != "" && c.BankAccountHolder != "" && c.AccessSecret != ""
}
func (c PetroRunConfig) Validate() error {
	if c.TestMode && os.Getenv("APP_NAME") != "petrolida-staging-api" {
		return fmt.Errorf("PETRORUN_TEST_MODE is only permitted for petrolida-staging-api")
	}
	keys := map[string]string{"PETRORUN_BASE_PRICE_IDR": os.Getenv("PETRORUN_BASE_PRICE_IDR"), "PETRORUN_ADMIN_FEE_IDR": os.Getenv("PETRORUN_ADMIN_FEE_IDR"), "PETRORUN_BANK_NAME": os.Getenv("PETRORUN_BANK_NAME"), "PETRORUN_BANK_ACCOUNT_NUMBER": os.Getenv("PETRORUN_BANK_ACCOUNT_NUMBER"), "PETRORUN_BANK_ACCOUNT_HOLDER": os.Getenv("PETRORUN_BANK_ACCOUNT_HOLDER"), "PETRORUN_ACCESS_SECRET": os.Getenv("PETRORUN_ACCESS_SECRET")}
	for _, size := range petroRunSurchargeSizes {
		key := "PETRORUN_JERSEY_SURCHARGE_" + size + "_IDR"
		keys[key] = os.Getenv(key)
	}
	for _, key := range petroRunSaleEnvKeys {
		keys[key] = os.Getenv(key)
	}
	if c.TestMode {
		keys["PETRORUN_TEST_MODE"] = "true"
	}
	set := false
	for _, v := range keys {
		if strings.TrimSpace(v) != "" {
			set = true
			break
		}
	}
	if !set {
		return nil
	}
	if !c.Available() {
		return fmt.Errorf("PETRORUN configuration is incomplete")
	}
	if c.BasePriceIDR < 0 || c.AdminFeeIDR < 0 {
		return fmt.Errorf("PETRORUN prices must be non-negative")
	}
	for size, surcharge := range c.JerseySurcharges {
		if surcharge < 0 {
			return fmt.Errorf("PETRORUN_JERSEY_SURCHARGE_%s_IDR must be non-negative", size)
		}
	}
	if len(c.AccessSecret) < 32 {
		return fmt.Errorf("PETRORUN_ACCESS_SECRET must contain at least 32 characters")
	}
	if c.Sales != nil {
		if err := validatePetroRunSales(*c.Sales); err != nil {
			return err
		}
	}
	return nil
}
func loadPetroRunConfig() (PetroRunConfig, error) {
	p, err := int64FromEnv("PETRORUN_BASE_PRICE_IDR", 0)
	if err != nil {
		return PetroRunConfig{}, err
	}
	f, err := int64FromEnv("PETRORUN_ADMIN_FEE_IDR", 0)
	if err != nil {
		return PetroRunConfig{}, err
	}
	surcharges := make(map[string]int64, len(petroRunSurchargeSizes))
	for _, size := range petroRunSurchargeSizes {
		key := "PETRORUN_JERSEY_SURCHARGE_" + size + "_IDR"
		surcharge, err := int64FromEnv(key, 0)
		if err != nil {
			return PetroRunConfig{}, err
		}
		surcharges[size] = surcharge
	}
	c := PetroRunConfig{
		BasePriceIDR:      p,
		AdminFeeIDR:       f,
		JerseySurcharges:  surcharges,
		BankName:          strings.TrimSpace(os.Getenv("PETRORUN_BANK_NAME")),
		BankAccountNumber: strings.TrimSpace(os.Getenv("PETRORUN_BANK_ACCOUNT_NUMBER")),
		BankAccountHolder: strings.TrimSpace(os.Getenv("PETRORUN_BANK_ACCOUNT_HOLDER")),
		AccessSecret:      strings.TrimSpace(os.Getenv("PETRORUN_ACCESS_SECRET")),
	}
	if raw := strings.TrimSpace(os.Getenv("PETRORUN_TEST_MODE")); raw != "" {
		c.TestMode, err = strconv.ParseBool(raw)
		if err != nil {
			return PetroRunConfig{}, fmt.Errorf("PETRORUN_TEST_MODE must be a boolean")
		}
	}
	c.Sales, err = loadPetroRunSales(p)
	if err != nil {
		return PetroRunConfig{}, err
	}
	return c, c.Validate()
}

type SizeConfig struct {
	Code         string `json:"code"`
	SurchargeIDR int64  `json:"surcharge_idr"`
	MaxChestCM   int    `json:"max_chest_cm"`
	MaxLengthCM  int    `json:"max_length_cm"`
}

var petroRunSurchargeSizes = []string{"2XL", "3XL", "4XL"}

var petroRunSizeDefinitions = []SizeConfig{{"S", 0, 48, 69}, {"M", 0, 50, 71}, {"L", 0, 53, 73}, {"XL", 0, 56, 75}, {"2XL", 0, 58, 77}, {"3XL", 0, 60, 79}, {"4XL", 0, 62, 81}}

func (c PetroRunConfig) AvailableSizes() []SizeConfig {
	sizes := make([]SizeConfig, len(petroRunSizeDefinitions))
	copy(sizes, petroRunSizeDefinitions)
	for i := range sizes {
		sizes[i].SurchargeIDR = c.JerseySurcharges[sizes[i].Code]
	}
	return sizes
}

func (c PetroRunConfig) JerseySize(code string) (SizeConfig, bool) {
	if strings.EqualFold(strings.TrimSpace(code), "XXL") {
		code = "2XL"
	}
	for _, s := range c.AvailableSizes() {
		if s.Code == strings.ToUpper(code) {
			return s, true
		}
	}
	return SizeConfig{}, false
}
