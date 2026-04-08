package id

import (
	"crypto/rand"
	"fmt"
	"math"
	"regexp"
	"strings"
)

var (
	// reUniqueID matches the native 32-char lowercase hex ID produced by UniqueID.
	reUniqueID = regexp.MustCompile(`^[a-f0-9]{32}$`)
	// reUUIDv4v7 matches canonical UUID v4 and v7 strings (RFC 4122 / RFC 9562):
	// xxxxxxxx-xxxx-[47]xxx-[89ab]xxx-xxxxxxxxxxxx
	reUUIDv4v7 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[47][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
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

// IsUniqueID returns true if s is the native 32-char hex ID produced by
// UniqueID (identified by format and entropy), or a canonical UUID v4 or v7 string.
func IsUniqueID(s string) bool {
	s = strings.ToLower(s)
	if reUUIDv4v7.MatchString(s) {
		return true
	}
	if !reUniqueID.MatchString(s) {
		return false
	}
	return Entropy(s) > 64
}

// Returns the Shannon entropy in bits
//
// Input string is either upper or lower case, not both.
func Entropy(s string) float64 {

	l := float64(len(s))

	freq := make(map[rune]float64)
	for _, c := range s {
		freq[c]++
	}
	var entropy float64
	for _, count := range freq {
		p := count / l
		entropy -= p * math.Log2(p)
	}
	return entropy * l
}
