package alias

import (
	"fmt"
	authpkg "internal/auth"
	. "pkg/math"
	_ "pkg/driver"
)

func example() {
	fmt.Println("ok")
	authpkg.ValidateToken("token")
	Add(1, 2)
}
