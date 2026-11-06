package utils

import (
	"net/url"
	"strings"
)

// IsGoogleDriveURL mengecek apakah value adalah link HTTPS ke folder Google Drive.
func IsGoogleDriveURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "drive.google.com" && host != "docs.google.com" {
		return false
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for index, segment := range segments {
		if segment == "folders" && index+1 < len(segments) && strings.TrimSpace(segments[index+1]) != "" {
			return true
		}
	}
	return false
}
