package vardecls

import "pkg/db"

type Client struct{}

func example() {
	var a db.Client
	var b *db.Client
	var c Client
	var d *Client
	e := db.Client{}
	f := Client{}
	_, _, _, _, _, _ = a, b, c, d, e, f
}
