package api

import (
	"github.com/wikismit/sample/internal/auth"
	"github.com/wikismit/sample/pkg/logger"
)

type Handler struct {
	Logger *logger.Logger
}

func Handle(token string) bool {
	logger.Info("handling request")
	return auth.ValidateToken(token)
}
