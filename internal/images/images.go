package images

import (
	"errors"
	"os"
	"path/filepath"
)

func ResolveOptionalPath(cwd, raw string) (string, bool, error) {
	if raw == "" {
		return "", false, nil
	}
	path := raw
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false, err
	}
	info, err := os.Stat(abs)
	if errors.Is(err, os.ErrNotExist) {
		return abs, false, nil
	}
	if err != nil {
		return "", false, err
	}
	if info.IsDir() {
		return abs, false, nil
	}
	return abs, true, nil
}

func TypeFromFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	header := make([]byte, 12)
	n, err := file.Read(header)
	if err != nil {
		return "", err
	}
	header = header[:n]
	return TypeFromBytes(header), nil
}

func TypeFromBytes(data []byte) string {
	switch {
	case len(data) >= 8 &&
		data[0] == 0x89 &&
		data[1] == 'P' &&
		data[2] == 'N' &&
		data[3] == 'G' &&
		data[4] == '\r' &&
		data[5] == '\n' &&
		data[6] == 0x1a &&
		data[7] == '\n':
		return "PNG"
	case len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
		return "JPG"
	case len(data) >= 6 && string(data[:3]) == "GIF":
		return "GIF"
	default:
		return ""
	}
}
