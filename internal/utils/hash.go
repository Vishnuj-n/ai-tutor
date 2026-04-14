package utils

import (
	"crypto/md5"
	"fmt"
)

// MD5Hex returns the lowercase hex MD5 digest for the provided text.
func MD5Hex(text string) string {
	hash := md5.Sum([]byte(text))
	return fmt.Sprintf("%x", hash)
}
