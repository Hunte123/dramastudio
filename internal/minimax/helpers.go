package minimax

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
)

func jsonReader(data []byte) *bytes.Reader { return bytes.NewReader(data) }

func writeFile(r io.Reader, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}
