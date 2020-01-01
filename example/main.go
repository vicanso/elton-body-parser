package main

import (
	"bytes"

	"github.com/vicanso/elton"
	bodyparser "github.com/vicanso/elton-body-parser"
)

func main() {
	e := elton.New()

	e.Use(bodyparser.NewDefault())

	e.POST("/user/login", func(c *elton.Context) (err error) {
		c.BodyBuffer = bytes.NewBuffer(c.RequestBody)
		return
	})

	err := e.ListenAndServe(":3000")
	if err != nil {
		panic(err)
	}
}
