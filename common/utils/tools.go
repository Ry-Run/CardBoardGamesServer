package utils

import (
	"math/rand"
	"time"
)

func Contains(data []string, include string) bool {
	for _, v := range data {
		if v == include {
			return true
		}
	}
	return false
}

func Rand(n int) int {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	return rand.Intn(n)
}
