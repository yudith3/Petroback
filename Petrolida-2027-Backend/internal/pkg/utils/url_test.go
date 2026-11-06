package utils

import "testing"

func TestIsGoogleDriveURL(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "Drive folder", value: "https://drive.google.com/drive/folders/example", want: true},
		{name: "Docs folder", value: "https://docs.google.com/drive/folders/example", want: true},
		{name: "Drive file", value: "https://drive.google.com/file/d/example/view", want: false},
		{name: "Empty folder", value: "https://drive.google.com/drive/folders/", want: false},
		{name: "Insecure", value: "http://drive.google.com/drive/folders/example", want: false},
		{name: "Lookalike host", value: "https://drive.google.com.example.test/drive/folders/example", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsGoogleDriveURL(test.value); got != test.want {
				t.Fatalf("IsGoogleDriveURL(%q) = %v, want %v", test.value, got, test.want)
			}
		})
	}
}
