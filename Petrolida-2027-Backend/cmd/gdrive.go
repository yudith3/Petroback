package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	driveAuthEndpoint   = "https://accounts.google.com/o/oauth2/v2/auth"
	driveTokenEndpoint  = "https://oauth2.googleapis.com/token"
	driveFilesEndpoint  = "https://www.googleapis.com/drive/v3/files"
	driveFileScope      = "https://www.googleapis.com/auth/drive.file"
	driveFolderMimeType = "application/vnd.google-apps.folder"

	driveConsentTimeout = 5 * time.Minute
	driveDefaultFolder  = "Portofolio Oprec Staff"

	driveCallbackPage = `<!doctype html><meta charset="utf-8"><title>Petrolida 2027</title>
<body style="font-family:system-ui;padding:3rem;text-align:center"><h2>%s</h2><p>%s</p></body>`
)

// runDriveToken menjalankan OAuth sekali jalan untuk menerbitkan refresh token
// dan folder penampung di Drive. Dipakai saat setup awal, dan saat kepengurusan
// berikutnya perlu menerbitkan token baru karena yang lama dicabut.
func runDriveToken() error {
	clientID := strings.TrimSpace(os.Getenv("GDRIVE_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("GDRIVE_CLIENT_SECRET"))
	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("GDRIVE_CLIENT_ID dan GDRIVE_CLIENT_SECRET harus diisi lebih dulu, lihat docs/gdrive-setup.md")
	}

	// Port dipilih sistem supaya tidak bentrok. OAuth client bertipe Desktop app
	// memperbolehkan redirect ke loopback di port mana pun.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("buka port callback: %w", err)
	}
	defer listener.Close()
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port)

	state, err := randomState()
	if err != nil {
		return err
	}

	codes := make(chan string, 1)
	failures := make(chan error, 1)
	server := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
		Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			query := request.URL.Query()
			writer.Header().Set("Content-Type", "text/html; charset=utf-8")

			switch {
			case query.Get("error") != "":
				fmt.Fprintf(writer, driveCallbackPage, "Gagal", "Google menolak: "+query.Get("error"))
				failures <- fmt.Errorf("google menolak permintaan: %s", query.Get("error"))
			case query.Get("state") != state:
				fmt.Fprintf(writer, driveCallbackPage, "Gagal", "State tidak cocok, proses dibatalkan.")
				failures <- fmt.Errorf("state tidak cocok, proses dibatalkan")
			case query.Get("code") != "":
				fmt.Fprintf(writer, driveCallbackPage, "Berhasil", "Silakan kembali ke terminal.")
				codes <- query.Get("code")
			}
		}),
	}
	go func() { _ = server.Serve(listener) }()
	defer func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
	}()

	authQuery := url.Values{}
	authQuery.Set("client_id", clientID)
	authQuery.Set("redirect_uri", redirectURI)
	authQuery.Set("response_type", "code")
	authQuery.Set("scope", driveFileScope)
	// Tanpa dua parameter ini Google hanya mengirim access token berumur sejam,
	// dan refresh token yang kita butuhkan tidak pernah terbit.
	authQuery.Set("access_type", "offline")
	authQuery.Set("prompt", "consent")
	authQuery.Set("state", state)
	authURL := driveAuthEndpoint + "?" + authQuery.Encode()

	fmt.Println("==================================================")
	fmt.Println("  Petrolida 2027 - Google Drive refresh token")
	fmt.Println("==================================================")
	fmt.Println()
	fmt.Println("Buka URL berikut, lalu login sebagai akun panitia pemilik Drive.")
	fmt.Println("Kalau muncul peringatan aplikasi belum diverifikasi, pilih Advanced")
	fmt.Println("lalu lanjutkan. Itu wajar untuk aplikasi internal.")
	fmt.Println()
	fmt.Println(authURL)
	fmt.Println()
	openBrowser(authURL)
	fmt.Println("Menunggu persetujuan di browser...")

	var code string
	select {
	case code = <-codes:
	case err := <-failures:
		return err
	case <-time.After(driveConsentTimeout):
		return fmt.Errorf("tidak ada persetujuan dalam %s, proses dibatalkan", driveConsentTimeout)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", redirectURI)

	var tokens struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := postForm(ctx, driveTokenEndpoint, form, &tokens); err != nil {
		return fmt.Errorf("tukar authorization code: %w", err)
	}
	if tokens.RefreshToken == "" {
		return fmt.Errorf("google tidak mengirim refresh token, ulangi prosesnya dari awal")
	}

	folderID, folderName, err := ensureDriveFolder(ctx, tokens.AccessToken)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("==================================================")
	fmt.Println("  Berhasil. Salin tiga baris ini ke .env")
	fmt.Println("==================================================")
	fmt.Println()
	fmt.Println("PORTFOLIO_STORAGE_DRIVER=gdrive")
	fmt.Printf("GDRIVE_REFRESH_TOKEN=%s\n", tokens.RefreshToken)
	fmt.Printf("GDRIVE_FOLDER_ID=%s\n", folderID)
	fmt.Println()
	fmt.Printf("Folder %q bisa dibuka di:\n", folderName)
	fmt.Printf("https://drive.google.com/drive/folders/%s\n", folderID)
	fmt.Println()
	fmt.Println("Kalau aplikasinya masih berstatus Testing di Google Auth Platform,")
	fmt.Println("ubah ke In production. Refresh token dari aplikasi berstatus Testing")
	fmt.Println("mati dalam 7 hari dan pendaftaran akan berhenti mendadak.")
	return nil
}

// ensureDriveFolder memakai folder yang sudah ada kalau GDRIVE_FOLDER_ID diisi,
// selain itu membuat yang baru. Foldernya wajib dibuat lewat API karena scope
// drive.file hanya mengizinkan aplikasi menyentuh berkas buatannya sendiri,
// sehingga folder yang dibuat manual lewat tampilan Drive akan berbalas 404.
func ensureDriveFolder(ctx context.Context, accessToken string) (string, string, error) {
	var folder struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	if existing := strings.TrimSpace(os.Getenv("GDRIVE_FOLDER_ID")); existing != "" {
		endpoint := driveFilesEndpoint + "/" + url.PathEscape(existing) + "?fields=id,name"
		if err := driveRequest(ctx, http.MethodGet, endpoint, accessToken, nil, &folder); err != nil {
			return "", "", fmt.Errorf("folder %s tidak bisa diakses aplikasi ini, "+
				"kemungkinan dibuat manual lewat Drive dan bukan oleh aplikasi: %w", existing, err)
		}
		return folder.ID, folder.Name, nil
	}

	name := strings.TrimSpace(os.Getenv("GDRIVE_FOLDER_NAME"))
	if name == "" {
		name = fmt.Sprintf("%s %d", driveDefaultFolder, time.Now().Year()+1)
	}
	body, err := json.Marshal(map[string]string{"name": name, "mimeType": driveFolderMimeType})
	if err != nil {
		return "", "", err
	}
	if err := driveRequest(
		ctx, http.MethodPost, driveFilesEndpoint+"?fields=id,name", accessToken, body, &folder,
	); err != nil {
		return "", "", fmt.Errorf("buat folder drive: %w", err)
	}
	return folder.ID, folder.Name, nil
}

func driveRequest(ctx context.Context, method, endpoint, accessToken string, body []byte, out any) error {
	var reader io.Reader
	if body != nil {
		reader = strings.NewReader(string(body))
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	if body != nil {
		request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return fmt.Errorf("google membalas %d: %s", response.StatusCode, strings.TrimSpace(string(detail)))
	}
	return json.NewDecoder(response.Body).Decode(out)
}

func postForm(ctx context.Context, endpoint string, form url.Values, out any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return fmt.Errorf("google membalas %d: %s", response.StatusCode, strings.TrimSpace(string(detail)))
	}
	return json.NewDecoder(response.Body).Decode(out)
}

func randomState() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("buat state: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

func openBrowser(target string) {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", target)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	case "linux":
		command = exec.Command("xdg-open", target)
	default:
		return
	}
	_ = command.Start()
}
