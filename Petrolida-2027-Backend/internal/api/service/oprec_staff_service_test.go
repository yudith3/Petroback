package service

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	portfoliostorage "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/storage"
)

func TestConfigListsEachSocialFollowProof(t *testing.T) {
	service := &oprecStaffService{config: config.OprecConfig{}}
	got := service.Config(time.Now()).RequiredDocuments
	want := []string{
		"KTM",
		"CV",
		"twibbon",
		"proof_of_follow_linkedin",
		"proof_of_follow_instagram",
		"proof_of_follow_tiktok",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("required documents = %v, want %v", got, want)
	}
}

func TestConfigReturnsAnnouncementSchedule(t *testing.T) {
	announcementAt := time.Date(2026, time.November, 1, 12, 30, 0, 0, time.FixedZone("WIB", 7*60*60))
	service := &oprecStaffService{config: config.OprecConfig{AnnouncementAt: announcementAt}}

	before := service.Config(announcementAt.Add(-time.Nanosecond))
	if before.AnnouncementOpen {
		t.Fatal("announcement is open before its scheduled time")
	}
	if before.AnnouncementAt == nil || !before.AnnouncementAt.Equal(announcementAt) {
		t.Fatalf("announcement at = %v, want %v", before.AnnouncementAt, announcementAt)
	}

	atSchedule := service.Config(announcementAt)
	if !atSchedule.AnnouncementOpen {
		t.Fatal("announcement is closed at its scheduled time")
	}
}

func TestConfigKeepsUnscheduledAnnouncementClosed(t *testing.T) {
	service := &oprecStaffService{config: config.OprecConfig{}}

	got := service.Config(time.Now())
	if got.AnnouncementOpen {
		t.Fatal("unscheduled announcement is open")
	}
	if got.AnnouncementAt != nil {
		t.Fatalf("announcement at = %v, want nil", got.AnnouncementAt)
	}
}

func TestSavePortfolio(t *testing.T) {
	directory := t.TempDir()
	storage, err := portfoliostorage.NewPortfolioStorage(directory)
	if err != nil {
		t.Fatal(err)
	}
	service := &oprecStaffService{
		config:  config.OprecConfig{PortfolioMaxBytes: 1024 * 1024},
		storage: storage,
	}

	header := multipartFileHeader(t, "portfolio.pdf", readFixture(t, "portfolio.pdf"))
	key, mimeType, err := service.savePortfolio(header, "test.pdf")
	if err != nil {
		t.Fatalf("savePortfolio returned error: %v", err)
	}
	if mimeType != "application/pdf" {
		t.Fatalf("mimeType = %q, want application/pdf", mimeType)
	}
	if _, err := os.Stat(filepath.Join(directory, key)); err != nil {
		t.Fatalf("saved portfolio not found: %v", err)
	}
}

func TestSavePortfolioRejectsFakePDF(t *testing.T) {
	storage, err := portfoliostorage.NewPortfolioStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := &oprecStaffService{
		config:  config.OprecConfig{PortfolioMaxBytes: 1024 * 1024},
		storage: storage,
	}

	header := multipartFileHeader(t, "portfolio.pdf", []byte("not actually a PDF"))
	if _, _, err := service.savePortfolio(header, "test.pdf"); err == nil {
		t.Fatal("savePortfolio accepted a fake PDF")
	}
}

func multipartFileHeader(t *testing.T, name string, content []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("portfolio", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest("POST", "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := request.ParseMultipartForm(int64(body.Len())); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = request.MultipartForm.RemoveAll() })
	return request.MultipartForm.File["portfolio"][0]
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func TestPortfolioFileName(t *testing.T) {
	tests := []struct {
		name string
		nrp  string
		full string
		want string
	}{
		{name: "Normal", nrp: "5025251178", full: "Fazli Irham", want: "5025251178_Fazli_Irham.pdf"},
		{name: "Karakter aneh", nrp: "50/25#251178", full: "Fazli (Irham)", want: "50_25_251178_Fazli_Irham.pdf"},
		{name: "Tanpa NRP", nrp: "", full: "Fazli", want: "Fazli.pdf"},
		{name: "Tanpa nama", nrp: "5025251178", full: "", want: "5025251178.pdf"},
		{name: "Kosong semua", nrp: "", full: "", want: "portfolio.pdf"},
		{name: "Non latin", nrp: "5025251178", full: "こんにちは", want: "5025251178.pdf"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := portfolioFileName(test.nrp, test.full); got != test.want {
				t.Fatalf("portfolioFileName(%q, %q) = %q, want %q", test.nrp, test.full, got, test.want)
			}
		})
	}
}

func TestPortfolioFileNameTruncatesLongName(t *testing.T) {
	got := portfolioFileName("5025251178", strings.Repeat("a", 200))
	if len(got) > 80 {
		t.Fatalf("nama berkas terlalu panjang (%d): %q", len(got), got)
	}
	if !strings.HasSuffix(got, ".pdf") {
		t.Fatalf("nama berkas kehilangan ekstensi: %q", got)
	}
}
