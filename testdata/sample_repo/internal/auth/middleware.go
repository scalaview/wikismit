package auth

import "github.com/wikismit/sample/pkg/logger"

func Middleware(token string) bool {
	logger.Info("running middleware")
	return ValidateToken(token)
}
