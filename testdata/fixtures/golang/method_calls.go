package methodcalls

import "pkg/db"

type Client struct{}

func (c *Client) Process() {
	c.execute()
}

func (c *Client) execute() {}

func example() {
	var c db.Client
	c.Query("SELECT 1")
}

func local() {
	var c Client
	c.execute()
}
