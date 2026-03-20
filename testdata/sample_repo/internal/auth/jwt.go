package auth

import (
	"github.com/wikismit/sample/pkg/errors"
	"github.com/wikismit/sample/pkg/logger"
)

func GenerateToken(subject string) (string, error) {
	if subject == "" {
		return "", errors.New("empty subject")
	}
	logger.Info("generating token")
	return subject + "-token", nil
}

func ValidateToken(token string) bool {
	logger.Info("validating token")
	return token != ""
}
