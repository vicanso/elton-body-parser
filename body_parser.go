// Copyright 2018 tree xie
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bodyparser

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/vicanso/elton"
	"github.com/vicanso/hes"
)

const (
	// ErrCategory body parser error category
	ErrCategory = "elton-body-parser"
	// 默认为50kb
	defaultRequestBodyLimit   = 50 * 1024
	jsonContentType           = "application/json"
	formURLEncodedContentType = "application/x-www-form-urlencoded"
)

type (
	// Decode body decode function
	Decode func(c *elton.Context, originalData []byte) (data []byte, err error)
	// Validate body content type check validate function
	Validate func(c *elton.Context) bool
	// Decoder decoder
	Decoder struct {
		Decode   Decode
		Validate Validate
	}
	// Config json parser config
	Config struct {
		// Limit the limit size of body
		Limit int
		// Decoders decode list
		Decoders            []*Decoder
		Skipper             elton.Skipper
		ContentTypeValidate Validate
	}
)

var (
	validMethods = []string{
		http.MethodPost,
		http.MethodPatch,
		http.MethodPut,
	}
	errInvalidJSON = &hes.Error{
		Category:   ErrCategory,
		Message:    "invalid json format",
		StatusCode: http.StatusBadRequest,
	}
	jsonBytes = []byte("{}[]")
)

// AddDecoder add decoder
func (conf *Config) AddDecoder(decoder *Decoder) {
	if len(conf.Decoders) == 0 {
		conf.Decoders = make([]*Decoder, 0)
	}
	conf.Decoders = append(conf.Decoders, decoder)
}

// NewGzipDecoder new gzip decoder
func NewGzipDecoder() *Decoder {
	return &Decoder{
		Validate: func(c *elton.Context) bool {
			encoding := c.GetRequestHeader(elton.HeaderContentEncoding)
			return encoding == elton.Gzip
		},
		Decode: func(c *elton.Context, originalData []byte) (data []byte, err error) {
			c.SetRequestHeader(elton.HeaderContentEncoding, "")
			return doGunzip(originalData)
		},
	}
}

// NewJSONDecoder new json decoder
func NewJSONDecoder() *Decoder {
	return &Decoder{
		Validate: func(c *elton.Context) bool {
			ct := c.GetRequestHeader(elton.HeaderContentType)
			ctFields := strings.Split(ct, ";")
			return ctFields[0] == jsonContentType
		},
		Decode: func(c *elton.Context, originalData []byte) (data []byte, err error) {
			originalData = bytes.TrimSpace(originalData)
			if len(originalData) == 0 {
				return nil, nil
			}
			firstByte := originalData[0]
			lastByte := originalData[len(originalData)-1]

			if firstByte != jsonBytes[0] && firstByte != jsonBytes[2] {
				err = errInvalidJSON
				return
			}
			if firstByte == jsonBytes[0] && lastByte != jsonBytes[1] {
				err = errInvalidJSON
				return
			}
			if firstByte == jsonBytes[2] && lastByte != jsonBytes[3] {
				err = errInvalidJSON
				return
			}
			return originalData, nil
		},
	}
}

// NewFormURLEncodedDecoder new form url encode decoder
func NewFormURLEncodedDecoder() *Decoder {
	return &Decoder{
		Validate: func(c *elton.Context) bool {
			ct := c.GetRequestHeader(elton.HeaderContentType)
			ctFields := strings.Split(ct, ";")
			return ctFields[0] == formURLEncodedContentType
		},
		Decode: func(c *elton.Context, originalData []byte) (data []byte, err error) {
			urlValues, err := url.ParseQuery(string(originalData))
			if err != nil {
				he := hes.Wrap(err)
				he.Exception = true
				return nil, he
			}

			arr := make([]string, 0, len(urlValues))
			for key, values := range urlValues {
				if len(values) < 2 {
					arr = append(arr, fmt.Sprintf(`"%s":"%s"`, key, values[0]))
					continue
				}
				tmpArr := []string{}
				for _, v := range values {
					tmpArr = append(tmpArr, `"`+v+`"`)
				}
				arr = append(arr, fmt.Sprintf(`"%s":[%s]`, key, strings.Join(tmpArr, ",")))
			}
			data = []byte("{" + strings.Join(arr, ",") + "}")
			return data, nil
		},
	}
}

// DefaultJSONContentTypeValidate default json content type validate
func DefaultJSONContentTypeValidate(c *elton.Context) bool {
	ct := c.GetRequestHeader(elton.HeaderContentType)
	return strings.HasPrefix(ct, jsonContentType)
}

// DefaultJSONAndFormContentTypeValidate default json and form content type validate
func DefaultJSONAndFormContentTypeValidate(c *elton.Context) bool {
	ct := c.GetRequestHeader(elton.HeaderContentType)
	if strings.HasPrefix(ct, jsonContentType) {
		return true
	}
	return strings.HasPrefix(ct, formURLEncodedContentType)
}

// NewDefault create a default body parser, default limit and only json parser
func NewDefault() elton.Handler {
	conf := Config{}
	conf.AddDecoder(NewGzipDecoder())
	conf.AddDecoder(NewJSONDecoder())
	return New(conf)
}

// doGunzip gunzip
func doGunzip(buf []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewBuffer(buf))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return ioutil.ReadAll(r)
}

type maxBytesReader struct {
	r   io.ReadCloser // underlying reader
	n   int64         // max bytes remaining
	err error         // sticky error
}

func (l *maxBytesReader) Read(p []byte) (n int, err error) {
	if l.err != nil {
		return 0, l.err
	}
	if len(p) == 0 {
		return 0, nil
	}
	// If they asked for a 32KB byte read but only 5 bytes are
	// remaining, no need to read 32KB. 6 bytes will answer the
	// question of the whether we hit the limit or go past it.
	if int64(len(p)) > l.n+1 {
		p = p[:l.n+1]
	}
	n, err = l.r.Read(p)

	if int64(n) <= l.n {
		l.n -= int64(n)
		l.err = err
		return n, err
	}

	l.err = fmt.Errorf("request body is too large, it should be <= %d", l.n)

	n = int(l.n)
	l.n = 0

	return n, l.err
}

func (l *maxBytesReader) Close() error {
	return l.r.Close()
}

func MaxBytesReader(r io.ReadCloser, n int64) *maxBytesReader {
	return &maxBytesReader{
		n: n,
		r: r,
	}
}

// New create a body parser
func New(config Config) elton.Handler {
	limit := defaultRequestBodyLimit
	if config.Limit != 0 {
		limit = config.Limit
	}
	skipper := config.Skipper
	if skipper == nil {
		skipper = elton.DefaultSkipper
	}
	contentTypeValidate := config.ContentTypeValidate
	if contentTypeValidate == nil {
		contentTypeValidate = DefaultJSONContentTypeValidate
	}
	return func(c *elton.Context) (err error) {
		if skipper(c) || c.RequestBody != nil || !contentTypeValidate(c) {
			return c.Next()
		}
		method := c.Request.Method

		// 对于非提交数据的method跳过
		valid := false
		for _, item := range validMethods {
			if item == method {
				valid = true
				break
			}
		}
		if !valid {
			return c.Next()
		}
		// 如果request body为空，则表示未读取数据
		if c.RequestBody == nil {
			r := c.Request.Body
			if limit > 0 {
				r = MaxBytesReader(r, int64(limit))
			}
			defer r.Close()
			body, e := ioutil.ReadAll(r)
			if e != nil {
				// IO 读取失败的认为是 exception
				err = &hes.Error{
					Exception:  true,
					StatusCode: http.StatusInternalServerError,
					Message:    e.Error(),
					Category:   ErrCategory,
					Err:        e,
				}
				return
			}
			c.Request.Body.Close()
			c.RequestBody = body
		}
		body := c.RequestBody

		decodeList := make([]Decode, 0)
		for _, decoder := range config.Decoders {
			if decoder.Validate(c) {
				decodeList = append(decodeList, decoder.Decode)
				break
			}
		}
		// 没有符合条件的解码
		if len(decodeList) == 0 {
			return c.Next()
		}

		for _, decode := range decodeList {
			body, err = decode(c, body)
			if err != nil {
				return
			}
		}
		c.RequestBody = body

		return c.Next()
	}
}
