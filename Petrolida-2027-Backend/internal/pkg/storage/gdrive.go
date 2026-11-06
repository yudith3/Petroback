package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	portfolioMimeType = "application/pdf"

	driveTokenEndpoint  = "https://oauth2.googleapis.com/token"
	driveFilesEndpoint  = "https://www.googleapis.com/drive/v3/files"
	driveUploadEndpoint = "https://www.googleapis.com/upload/drive/v3/files"

	// Access token Google berumur satu jam. Disegarkan lebih awal supaya tidak
	// ada unggahan yang jatuh tepat di detik pergantian.
	driveTokenEarlyRefresh = time.Minute
)

// DriveCredentials datang dari environment. Refresh token mewakili akun panitia
// yang menekan Allow saat setup, dan akun itulah pemilik semua berkas terunggah.
type DriveCredentials struct {
	ClientID     string
	ClientSecret string
	RefreshToken string
	FolderID     string
	Timeout      time.Duration
}

type DriveStorage struct {
	tokens   *driveTokenSource
	client   *http.Client
	folderID string
	timeout  time.Duration

	// Endpoint disimpan sebagai field, bukan dipakai langsung dari konstanta,
	// supaya test bisa mengarahkannya ke server tiruan.
	filesURL  string
	uploadURL string
}

func NewDriveStorage(credentials DriveCredentials) (*DriveStorage, error) {
	switch {
	case credentials.ClientID == "" || credentials.ClientSecret == "":
		return nil, errors.New("drive client id and secret are empty")
	case credentials.RefreshToken == "":
		return nil, errors.New("drive refresh token is empty")
	case credentials.FolderID == "":
		return nil, errors.New("drive folder id is empty")
	}

	timeout := credentials.Timeout
	if timeout <= 0 {
		timeout = time.Minute
	}
	client := &http.Client{}

	return &DriveStorage{
		tokens: &driveTokenSource{
			clientID:     credentials.ClientID,
			clientSecret: credentials.ClientSecret,
			refreshToken: credentials.RefreshToken,
			client:       client,
			endpoint:     driveTokenEndpoint,
		},
		client:    client,
		folderID:  credentials.FolderID,
		timeout:   timeout,
		filesURL:  driveFilesEndpoint,
		uploadURL: driveUploadEndpoint,
	}, nil
}

// Save memakai multipart upload supaya metadata dan isi berkas terkirim dalam
// satu permintaan. Panjang total dihitung dari size agar bisa dikirim dengan
// Content-Length, bukan chunked.
func (s *DriveStorage) Save(name string, size int64, reader io.Reader) (string, error) {
	if name == "" {
		name = "portfolio.pdf"
	}
	contentType := driveContentType(name)

	metadata, err := json.Marshal(map[string]any{
		"name":     name,
		"parents":  []string{s.folderID},
		"mimeType": contentType,
	})
	if err != nil {
		return "", fmt.Errorf("encode portfolio metadata: %w", err)
	}

	boundary := "petrolida-upload-" + uuid.NewString()
	head := "--" + boundary + "\r\nContent-Type: application/json; charset=UTF-8\r\n\r\n" +
		string(metadata) + "\r\n--" + boundary + "\r\nContent-Type: " + contentType + "\r\n\r\n"
	tail := "\r\n--" + boundary + "--\r\n"

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	endpoint := s.uploadURL + "?uploadType=multipart&supportsAllDrives=true&fields=id"
	body := io.MultiReader(strings.NewReader(head), reader, strings.NewReader(tail))
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "multipart/related; boundary="+boundary)
	request.ContentLength = int64(len(head)) + size + int64(len(tail))

	response, err := s.do(ctx, request)
	if err != nil {
		return "", fmt.Errorf("upload portfolio to drive: %w", err)
	}
	defer response.Body.Close()

	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		return "", fmt.Errorf("decode drive response: %w", err)
	}
	if created.ID == "" {
		return "", errors.New("drive did not return a file id")
	}
	return created.ID, nil
}

func driveContentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	default:
		return portfolioMimeType
	}
}

func (s *DriveStorage) Open(key string) (io.ReadCloser, error) {
	if key == "" {
		return nil, errors.New("invalid portfolio key")
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	endpoint := s.filesURL + "/" + url.PathEscape(key) + "?alt=media&supportsAllDrives=true"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		cancel()
		return nil, err
	}

	response, err := s.do(ctx, request)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("download portfolio from drive: %w", err)
	}
	// Context dibatalkan saat body ditutup, bukan saat fungsi ini selesai, karena
	// unduhannya masih berjalan selama body dibaca handler.
	return &cancelOnClose{body: response.Body, cancel: cancel}, nil
}

func (s *DriveStorage) Delete(key string) error {
	if key == "" {
		return errors.New("invalid portfolio key")
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	endpoint := s.filesURL + "/" + url.PathEscape(key) + "?supportsAllDrives=true"
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}

	response, err := s.do(ctx, request)
	if err != nil {
		// Berkas yang memang sudah tidak ada bukan kegagalan, karena Delete
		// dipanggil saat membersihkan pendaftaran yang gagal disimpan.
		var statusErr *driveStatusError
		if errors.As(err, &statusErr) && statusErr.status == http.StatusNotFound {
			return nil
		}
		return fmt.Errorf("delete portfolio from drive: %w", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	return nil
}

// do memasang Authorization lalu memeriksa status. Body hanya dikembalikan saat
// sukses, supaya pemanggil tidak perlu mengurus penutupan body pada jalur error.
func (s *DriveStorage) do(ctx context.Context, request *http.Request) (*http.Response, error) {
	token, err := s.tokens.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := s.client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		defer response.Body.Close()
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return nil, &driveStatusError{status: response.StatusCode, body: strings.TrimSpace(string(detail))}
	}
	return response, nil
}

type driveStatusError struct {
	status int
	body   string
}

func (e *driveStatusError) Error() string {
	return fmt.Sprintf("drive responded %d: %s", e.status, e.body)
}

// driveTokenSource menukar refresh token jadi access token dan menyimpannya
// sampai hampir kedaluwarsa. Aman dipakai banyak request sekaligus.
type driveTokenSource struct {
	clientID     string
	clientSecret string
	refreshToken string
	client       *http.Client
	endpoint     string

	mutex     sync.Mutex
	token     string
	expiresAt time.Time
}

func (t *driveTokenSource) accessToken(ctx context.Context) (string, error) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.token != "" && time.Now().Before(t.expiresAt) {
		return t.token, nil
	}

	form := url.Values{}
	form.Set("client_id", t.clientID)
	form.Set("client_secret", t.clientSecret)
	form.Set("refresh_token", t.refreshToken)
	form.Set("grant_type", "refresh_token")

	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, t.endpoint, strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := t.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("refresh drive access token: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		// invalid_grant di sini hampir selalu berarti refresh token sudah dicabut
		// atau aplikasinya masih berstatus Testing sehingga token mati tiap 7 hari.
		return "", fmt.Errorf("refresh drive access token, google responded %d: %s",
			response.StatusCode, strings.TrimSpace(string(detail)))
	}

	var payload struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode drive token response: %w", err)
	}
	if payload.AccessToken == "" {
		return "", errors.New("google did not return an access token")
	}

	lifetime := time.Duration(payload.ExpiresIn) * time.Second
	if lifetime > driveTokenEarlyRefresh {
		lifetime -= driveTokenEarlyRefresh
	}
	t.token = payload.AccessToken
	t.expiresAt = time.Now().Add(lifetime)
	return t.token, nil
}

type cancelOnClose struct {
	body   io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnClose) Read(p []byte) (int, error) {
	return c.body.Read(p)
}

func (c *cancelOnClose) Close() error {
	err := c.body.Close()
	c.cancel()
	return err
}
