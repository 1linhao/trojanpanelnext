package middleware

import (
	"github.com/sirupsen/logrus"
	"testing"
)

func TestSecretRedactionHookRedactsMessageAndFields(t *testing.T) {
	entry := &logrus.Entry{
		Message: "connect password=very-secret",
		Data:    logrus.Fields{"dsn": "user:very-secret@tcp"},
	}
	if err := newSecretRedactionHook("very-secret").Fire(entry); err != nil {
		t.Fatal(err)
	}
	if entry.Message != "connect password=[REDACTED]" || entry.Data["dsn"] != "user:[REDACTED]@tcp" {
		t.Fatalf("secret was not redacted: %#v", entry)
	}
}
