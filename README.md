# elton-body-parser

[![Build Status](https://img.shields.io/travis/vicanso/elton-body-parser.svg?label=linux+build)](https://travis-ci.org/vicanso/elton-body-parser)

Body parser for elton. It support `application/json` and `application/x-www-form-urlencoded` type, but `NewDefault` just support `application/json`.

```go
package main

import (
	"bytes"

	"github.com/vicanso/elton"
	bodyparser "github.com/vicanso/elton-body-parser"
)

func main() {
	d := elton.New()

	d.Use(bodyparser.NewDefault())

	d.POST("/user/login", func(c *elton.Context) (err error) {
		c.BodyBuffer = bytes.NewBuffer(c.RequestBody)
		return
	})

	d.ListenAndServe(":7001")
}
```