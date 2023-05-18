package utils

import "math/rand"

func GeneratePostId() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-"
	var ans = make([]byte, 10)
	for i := range ans {
		ans[i] = alphabet[rand.Intn(len(alphabet))]
	}
	return string(ans)
}
