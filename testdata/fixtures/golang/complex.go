package sample

import (
	"context"
	"fmt"
)

type Service interface {
	Run(context.Context) error
}

type Alias = string

type Widget struct{}

func (w *Widget) Run(ctx context.Context) error {
	_ = ctx
	return nil
}

func Build(name string) (string, error) {
	return fmt.Sprintf("hello %s", name), nil
}
