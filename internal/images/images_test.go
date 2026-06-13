package images

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTypeFromFileUsesContentNotExtension(t *testing.T) {
	imageType, err := TypeFromFile(filepath.Join("..", "..", "tests", "fixtures", "logo-engenere.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if imageType != "PNG" {
		t.Fatalf("image type = %q", imageType)
	}
}

func TestTypeFromBytes(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "tests", "fixtures", "logo-engenere.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if imageType := TypeFromBytes(data); imageType != "PNG" {
		t.Fatalf("image type = %q", imageType)
	}
	if imageType := TypeFromBytes([]byte("not an image")); imageType != "" {
		t.Fatalf("non-image type = %q", imageType)
	}
}
