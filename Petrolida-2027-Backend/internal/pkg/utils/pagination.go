package utils

// TotalPages menghitung jumlah halaman (pembagian dibulatkan ke atas).
// Mengembalikan 0 kalau pageSize tidak valid.
func TotalPages(totalItems int64, pageSize int) int {
	if pageSize <= 0 {
		return 0
	}
	return int((totalItems + int64(pageSize) - 1) / int64(pageSize))
}
