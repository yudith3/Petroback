package utils

import "strings"

// NullableString mengembalikan pointer ke value yang sudah di-trim,
// atau nil kalau value kosong setelah di-trim.
func NullableString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// ContainsFold mengecek apakah target ada di values tanpa membedakan huruf
// besar-kecil dan spasi di awal/akhir.
func ContainsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}
