package main

import (
	"os"

	"path/filepath"
)

func getRandomBytesSystem(path string, n int) ([]byte, error) {
	fh, err := os.Open(filepath.Clean(path))
	if err != nil {
		return []byte{}, err
	}
	defer fh.Close()

	out := make([]byte, n)
	r, err := fh.Read(out)
	if err != nil {
		return []byte{}, err
	}

	return out[:r], nil
}
