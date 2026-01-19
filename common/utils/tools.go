package utils

import (
	"math/rand"
	"time"
)

func Contains[T int | string](data []T, include T) bool {
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
