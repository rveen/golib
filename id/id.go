package id

import (
	"crypto/rand"
	"fmt"
)

// 128 bit UUID
func UniqueID() string {
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", bytes)
}
