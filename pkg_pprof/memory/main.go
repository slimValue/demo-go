package main

import (
	"bytes"
	"github.com/pkg/profile"
	"math/rand"
)

const Letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func generate(n int) string {
	var buf bytes.Buffer
	for i := 0; i < n; i++ {
		buf.WriteByte(Letters[rand.Intn(len(Letters))])
	}
	return buf.String()
}

func repeat(s string, n int) string {
	var result string
	for i := 0; i < n; i++ {
		result += s
	}

	return result
}

func main() {
	defer profile.Start(profile.MemProfile).Stop()

	for i := 0; i < 100; i++ {
		repeat(generate(100), 100)
	}
}
