package db

import (
	"github.com/wikismit/sample/pkg/errors"
	"github.com/wikismit/sample/pkg/logger"
)

type Client struct {
	DSN string
}

func Query(query string) (string, error) {
	if query == "" {
		return "", errors.New("empty query")
	}
	logger.Info("query executed")
	return query, nil
}
