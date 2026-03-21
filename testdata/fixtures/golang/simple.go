package sample

import "fmt"

type Widget struct{}

func Exported(name string) string {
	return fmt.Sprint(name)
}

func hidden() {}
