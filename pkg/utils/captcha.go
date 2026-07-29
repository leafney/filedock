package utils

import (
	"math/rand"
	"time"
)

const (
	// 数字字符集
	Digits = "0123456789"
	// 邀请码字符集（排除易混淆字符：0/O, 1/I/l, 8/B 等）
	InviteChars = "2345679ACDEFGHJKMNPQRSTUVWXYZ"
)

// GenerateRandomCaptcha 生成随机数字验证码
func GenerateRandomCaptcha(length int) string {
	if length <= 0 {
		length = 6
	}
	return generateRandom(length, Digits)
}

// GenerateRandomString 生成指定长度的随机字符串（使用邀请码字符集）
func GenerateRandomString(length int) string {
	if length <= 0 {
		length = 6
	}
	return generateRandom(length, InviteChars)
}

func generateRandom(length int, charset string) string {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	result := make([]byte, length)
	charsetLen := len(charset)

	for i := 0; i < length; i++ {
		result[i] = charset[rng.Intn(charsetLen)]
	}

	return string(result)
}
