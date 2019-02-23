# cod-body-parser

Body parser for cod.

```go
package main

import (
	"bytes"

	"github.com/vicanso/cod"
	bodyparser "github.com/vicanso/cod-body-parser"
)

func main() {
	d := cod.New()
	d.Keys = []string{
		"cuttlefish",
	}

	d.Use(bodyparser.NewDefault())

	d.POST("/user/login", func(c *cod.Context) (err error) {
		c.BodyBuffer = bytes.NewBuffer(c.RequestBody)
		return
	})

	d.ListenAndServe(":7001")
}

```