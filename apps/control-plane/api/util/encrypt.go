package util

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
)

const saltSize = 16

// Sha1String 加盐
func Sha1String(plain string) string {
	buf := make([]byte, saltSize, saltSize+sha1.Size)
	_, err := io.ReadFull(rand.Reader, buf)
	if err != nil {
		logrus.Errorf("random read failed err: %v", err)
	}
	h := sha1.New()
	h.Write(buf)
	h.Write([]byte(plain))
	return base64.URLEncoding.EncodeToString(h.Sum(buf))
}

// Sha1Match 匹配
func Sha1Match(secret, plain string) bool {
	data, _ := base64.URLEncoding.DecodeString(secret)
	if len(data) != saltSize+sha1.Size {
		logrus.Errorf("wrong length of data\n")
		return false
	}
	h := sha1.New()
	h.Write(data[:saltSize])
	h.Write([]byte(plain))
	return bytes.Equal(h.Sum(nil), data[saltSize:])
}

func SHA224String(password string) string {
	hash := sha256.New224()
	hash.Write([]byte(password))
	val := hash.Sum(nil)
	str := ""
	for _, v := range val {
		str += fmt.Sprintf("%02x", v)
	}
	return str
}
