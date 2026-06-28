package utils

import (
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// MD5Hex returns the lowercase hex MD5 digest for the provided text.
func MD5Hex(text string) string {
	hash := md5.Sum([]byte(text))
	return fmt.Sprintf("%x", hash)
}

// FileSHA256 returns the lowercase hex SHA-256 digest of a file's contents.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
