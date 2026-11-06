package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// Store adalah kontrak penyimpanan portofolio. Ada dua implementasi, disk lokal
// untuk development dan Google Drive untuk produksi, dipilih lewat
// PORTFOLIO_STORAGE_DRIVER.
type Store interface {
	// Save menyimpan isi reader lalu mengembalikan key untuk membukanya lagi.
	// Argumen name hanya nama tampilan, bukan key, karena penyimpanan lokal
	// memakai UUID sedangkan Drive memakai file ID buatan Google. Argumen size
	// dipakai Drive untuk menghitung Content-Length unggahan.
	Save(name string, size int64, reader io.Reader) (string, error)
	Open(key string) (io.ReadCloser, error)
	Delete(key string) error
}

type PortfolioStorage struct {
	directory string
}

func NewPortfolioStorage(directory string) (*PortfolioStorage, error) {
	clean := filepath.Clean(directory)
	if err := os.MkdirAll(clean, 0o700); err != nil {
		return nil, fmt.Errorf("create portfolio directory: %w", err)
	}
	return &PortfolioStorage{directory: clean}, nil
}

// Save mengabaikan name karena nama asli berkas sudah tersimpan di kolom
// original_name, sedangkan nama di disk sengaja dibuat acak.
func (s *PortfolioStorage) Save(name string, _ int64, reader io.Reader) (string, error) {
	extension := strings.ToLower(filepath.Ext(name))
	switch extension {
	case ".jpg", ".jpeg", ".png":
	default:
		extension = ".pdf"
	}
	key := uuid.NewString() + extension
	path := filepath.Join(s.directory, key)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("create portfolio: %w", err)
	}

	if _, err := io.Copy(file, reader); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("save portfolio: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close portfolio: %w", err)
	}
	return key, nil
}

func (s *PortfolioStorage) Delete(key string) error {
	path, err := s.pathFor(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *PortfolioStorage) Open(key string) (io.ReadCloser, error) {
	path, err := s.pathFor(key)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (s *PortfolioStorage) pathFor(key string) (string, error) {
	if key == "" || filepath.Base(key) != key || strings.ContainsAny(key, `/\\`) {
		return "", fmt.Errorf("invalid portfolio key")
	}
	return filepath.Join(s.directory, key), nil
}
