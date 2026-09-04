package util

import (
	"crypto/rand"
	"encoding/base64"
	"golang.org/x/crypto/curve25519"
)

func ExecuteX25519() (string, string, error) {
	// 生成一个随机的32字节私钥
	privateKey := make([]byte, curve25519.ScalarSize)
	if _, err := rand.Read(privateKey); err != nil {
		return "", "", err
	}

	// Modify random bytes using algorithm described at:
	// https://cr.yp.to/ecdh.html.
	privateKey[0] &= 248
	privateKey[31] &= 127
	privateKey[31] |= 64

	publicKey, err := curve25519.X25519(privateKey, curve25519.Basepoint)
	if err != nil {
		return "", "", err
	}

	return base64.RawURLEncoding.EncodeToString(publicKey), base64.RawURLEncoding.EncodeToString(privateKey), nil
}
