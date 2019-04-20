# cod-body-parser

[![Build Status](https://img.shields.io/travis/vicanso/cod-body-parser.svg?label=linux+build)](https://travis-ci.org/vicanso/cod-body-parser)

Body parser for cod. It support `application/json` and `application/x-www-form-urlencoded` type, but `NewDefault` just support `application/json`.

```go
package main

import (
	"bytes"

	"github.com/vicanso/cod"
	bodyparser "github.com/vicanso/cod-body-parser"
)

func main() {
	d := cod.New()

	d.Use(bodyparser.NewDefault())

	d.POST("/user/login", func(c *cod.Context) (err error) {
		c.BodyBuffer = bytes.NewBuffer(c.RequestBody)
		return
	})

	d.ListenAndServe(":7001")
}
```