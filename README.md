# elton-body-parser

The middleware has been archived, please use the middleware of [elton](https://github.com/vicanso/elton).

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
	e := elton.New()

	e.Use(bodyparser.NewDefault())

	e.POST("/user/login", func(c *elton.Context) (err error) {
		c.BodyBuffer = bytes.NewBuffer(c.RequestBody)
		return
	})

	e.ListenAndServe(":3000")
}
```

## API

### NewDefault

create a new default body parser middleware. It include gzip and json decoder.

```go
e.Use(bodyparser.NewDefault())
```

### NewGzipDecoder

create a new gzip decoder

```go
conf := bodyparser.Config{}
conf.AddDecoder(bodyparser.NewGzipDecoder())
e.Use(bodyparser.New(conf))
```

### NewJSONDecoder

create a new json decoder

```go
conf := bodyparser.Config{}
conf.AddDecoder(bodyparser.NewJSONDecoder())
e.Use(bodyparser.New(conf))
```

### NewFormURLEncodedDecoder

create a new form url encoded decoder

```go
conf := bodyparser.Config{
	ContentTypeValidate: bodyparser.DefaultJSONAndFormContentTypeValidate
}
conf.AddDecoder(bodyparser.NewFormURLEncodedDecoder())
e.Use(bodyparser.New(conf))
```
