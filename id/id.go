package id

import (
	"crypto/rand"
	"fmt"
	"math"
	"regexp"
	"strings"
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

// Return true if the input string is a probable UniqueID
func IsUniqueID(s string) bool {
	s = strings.ToLower(s)
	// TODO: compile this beforehand
	match, _ := regexp.MatchString(`^[a-f0-9]{32}$`, s)
	if !match {
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
