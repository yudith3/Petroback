package utils

import (
	"strconv"
)

// DefaultTake returns a default value for the take parameter if the provided value is less than or equal to 0.
// The default take value is 10.
func DefaultTake(i int) int {
	if i <= 0 {
		return 10
	}

	return i
}

// ToInt converts a string to an integer.
// If the conversion fails, it returns 0.
func ToInt(i string) int {
	res, err := strconv.Atoi(i)
	if err != nil {
		return 0
	}
	return res
}

// ToLinkDownload converts a fileId from gdrive to link download
// func ToLinkDownload(fileId string) string {
// 	link := fmt.Sprintf("https://drive.google.com/uc?id=%s&export=download", fileId)
// 	return link
// }

// func ToLinkFolder(folderId string) string {
// 	link := fmt.Sprintf("https://drive.google.com/drive/folders/%s?hl=id", folderId)
// 	return link
// }

// func ToOnlyEvent(eventname string) string {
// 	re := regexp.MustCompile(`\s*\(.*\)`)
// 	name := re.ReplaceAllString(eventname, "")
// 	name = strings.ReplaceAll(name, " Petrolida 2025", "")
// 	name = strings.ReplaceAll(name, " Petrolida 2026", "")
// 	return name
// }

// func ToPathAws(link string) string {
// 	prefix := "https://petrolida2025.s3.ap-southeast-2.amazonaws.com"
// 	if strings.HasPrefix(link, prefix) {
// 		return strings.TrimPrefix(link, prefix)
// 	}
// 	return link
// }
