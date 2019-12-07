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

	err := d.ListenAndServe(":3000")
	if err != nil {
		panic(err)
	}
}
