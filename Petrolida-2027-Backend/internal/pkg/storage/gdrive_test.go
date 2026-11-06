package storage

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestDrive membangun DriveStorage yang menunjuk ke server tiruan, lengkap
// dengan endpoint token, supaya jalur unggah dan unduh bisa diuji tanpa Google.
func newTestDrive(t *testing.T, handler http.Handler) (*DriveStorage, *httptest.Server, *int64) {
	t.Helper()

	var tokenCalls int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/token" {
			atomic.AddInt64(&tokenCalls, 1)
			writer.Header().Set("Content-Type", "application/json")
			fmt.Fprint(writer, `{"access_token":"test-access-token","expires_in":3600}`)
			return
		}
		handler.ServeHTTP(writer, request)
	}))
	t.Cleanup(server.Close)

	client := server.Client()
	storage := &DriveStorage{
		tokens: &driveTokenSource{
			clientID: "id", clientSecret: "secret", refreshToken: "1//refresh",
			client: client, endpoint: server.URL + "/token",
		},
		client:    client,
		folderID:  "folder-123",
		timeout:   10 * time.Second,
		filesURL:  server.URL + "/drive/v3/files",
		uploadURL: server.URL + "/upload/drive/v3/files",
	}
	return storage, server, &tokenCalls
}

func TestDriveSaveSendsMetadataAndContent(t *testing.T) {
	content := "%PDF-1.7 isi portofolio"

	var gotName, gotParent, gotBody, gotAuth string
	var gotLength int64
	storage, _, _ := newTestDrive(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotAuth = request.Header.Get("Authorization")
		gotLength = request.ContentLength

		mediaType, params, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
		if err != nil || mediaType != "multipart/related" {
			t.Errorf("Content-Type = %q, want multipart/related", request.Header.Get("Content-Type"))
			return
		}

		reader := multipart.NewReader(request.Body, params["boundary"])
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Errorf("baca part: %v", err)
				return
			}
			payload, _ := io.ReadAll(part)
			if strings.HasPrefix(part.Header.Get("Content-Type"), "application/json") {
				if strings.Contains(string(payload), `"name":"5025251178_Fazli.pdf"`) {
					gotName = "5025251178_Fazli.pdf"
				}
				if strings.Contains(string(payload), `"folder-123"`) {
					gotParent = "folder-123"
				}
				continue
			}
			gotBody = string(payload)
		}

		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"id":"drive-file-id"}`)
	}))

	key, err := storage.Save("5025251178_Fazli.pdf", int64(len(content)), strings.NewReader(content))
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if key != "drive-file-id" {
		t.Fatalf("key = %q, want drive-file-id", key)
	}
	if gotName != "5025251178_Fazli.pdf" {
		t.Error("nama berkas tidak terkirim di metadata")
	}
	if gotParent != "folder-123" {
		t.Error("folder tujuan tidak terkirim di metadata")
	}
	if gotBody != content {
		t.Errorf("isi terkirim = %q, want %q", gotBody, content)
	}
	if gotAuth != "Bearer test-access-token" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	// Content-Length wajib pasti, kalau nol berarti jatuh ke chunked encoding.
	if gotLength <= int64(len(content)) {
		t.Errorf("ContentLength = %d, harus lebih besar dari panjang isi (%d)", gotLength, len(content))
	}
}

func TestDriveSaveReportsFailure(t *testing.T) {
	storage, _, _ := newTestDrive(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
		fmt.Fprint(writer, `{"error":{"message":"storageQuotaExceeded"}}`)
	}))

	if _, err := storage.Save("a.pdf", 3, strings.NewReader("abc")); err == nil {
		t.Fatal("Save() menerima balasan 403 sebagai sukses")
	} else if !strings.Contains(err.Error(), "storageQuotaExceeded") {
		t.Errorf("pesan error tidak membawa alasan dari Google: %v", err)
	}
}

func TestDriveContentTypeSupportsPaymentProofImages(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "proof.jpg", want: "image/jpeg"},
		{name: "proof.jpeg", want: "image/jpeg"},
		{name: "proof.png", want: "image/png"},
		{name: "portfolio.pdf", want: "application/pdf"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := driveContentType(test.name); got != test.want {
				t.Errorf("driveContentType(%q) = %q, want %q", test.name, got, test.want)
			}
		})
	}
}

func TestDriveOpenStreamsContent(t *testing.T) {
	storage, _, _ := newTestDrive(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("alt") != "media" {
			t.Errorf("alt = %q, want media", request.URL.Query().Get("alt"))
		}
		fmt.Fprint(writer, "%PDF-1.7 unduhan")
	}))

	body, err := storage.Open("drive-file-id")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer body.Close()

	content, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("baca isi: %v", err)
	}
	if string(content) != "%PDF-1.7 unduhan" {
		t.Errorf("isi = %q", content)
	}
}

func TestDriveDeleteTreatsMissingFileAsSuccess(t *testing.T) {
	storage, _, _ := newTestDrive(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
	}))

	if err := storage.Delete("sudah-hilang"); err != nil {
		t.Fatalf("Delete() pada berkas yang tidak ada harus sukses, dapat: %v", err)
	}
}

func TestDriveReusesAccessTokenUntilExpiry(t *testing.T) {
	storage, _, tokenCalls := newTestDrive(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"id":"x"}`)
	}))

	for i := 0; i < 3; i++ {
		if _, err := storage.Save("a.pdf", 3, strings.NewReader("abc")); err != nil {
			t.Fatalf("Save() ke-%d error = %v", i, err)
		}
	}
	if got := *tokenCalls; got != 1 {
		t.Fatalf("token diminta %d kali, harusnya sekali lalu dipakai ulang", got)
	}
}

func TestNewDriveStorageRejectsIncompleteCredentials(t *testing.T) {
	tests := []struct {
		name        string
		credentials DriveCredentials
	}{
		{"tanpa client", DriveCredentials{RefreshToken: "1//a", FolderID: "f"}},
		{"tanpa refresh token", DriveCredentials{ClientID: "a", ClientSecret: "b", FolderID: "f"}},
		{"tanpa folder", DriveCredentials{ClientID: "a", ClientSecret: "b", RefreshToken: "1//a"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewDriveStorage(test.credentials); err == nil {
				t.Fatal("kredensial tidak lengkap diterima")
			}
		})
	}
}
