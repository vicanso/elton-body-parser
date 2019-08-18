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
	// Config json parser config
	Config struct {
		// Limit the limit size of body
		Limit int
		// IgnoreJSON ignore json type
		IgnoreJSON bool
		// IgnoreFormURLEncoded ignore form url encoded type
		IgnoreFormURLEncoded bool
		// Decode decode function
		Decode  Decode
		Skipper elton.Skipper
	}
)

var (
	validMethods = []string{
		http.MethodPost,
		http.MethodPatch,
		http.MethodPut,
	}
)

// DefaultDecode default decode
func DefaultDecode(c *elton.Context, data []byte) ([]byte, error) {
	encoding := c.GetRequestHeader(elton.HeaderContentEncoding)
	if encoding == elton.Gzip {
		c.SetRequestHeader(elton.HeaderContentEncoding, "")
		return doGunzip(data)
	}
	return data, nil
}

// NewDefault create a default body parser, default limit and only json parser
func NewDefault() elton.Handler {
	return New(Config{
		IgnoreFormURLEncoded: true,
		Decode:               DefaultDecode,
	})
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
	return func(c *elton.Context) (err error) {
		if skipper(c) || c.RequestBody != nil {
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
		ct := c.GetRequestHeader(elton.HeaderContentType)
		ctFields := strings.Split(ct, ";")
		// 非json则跳过
		isJSON := ctFields[0] == jsonContentType
		isFormURLEncoded := ctFields[0] == formURLEncodedContentType

		// 如果不是 json 也不是 form url encoded，则跳过
		if !isJSON && !isFormURLEncoded {
			return c.Next()
		}
		// 如果数据类型json，而且中间件不处理，则跳过
		if isJSON && config.IgnoreJSON {
			return c.Next()
		}

		// 如果数据类型form url encoded，而且中间件不处理，则跳过
		if isFormURLEncoded && config.IgnoreFormURLEncoded {
			return c.Next()
		}

		body, e := ioutil.ReadAll(c.Request.Body)
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
		if limit > 0 && len(body) > limit {
			err = &hes.Error{
				StatusCode: http.StatusBadRequest,
				Message:    fmt.Sprintf("request body is %d bytes, it should be <= %d", len(body), limit),
				Category:   ErrCategory,
			}
			return
		}
		if config.Decode != nil {
			// 对提交数据做decode处理（如解压，编码转换等）
			body, err = config.Decode(c, body)
			if err != nil {
				return
			}
		}
		// 将form url encoded 数据转化为json
		if isFormURLEncoded {
			data, err := url.ParseQuery(string(body))
			if err != nil {
				he := hes.Wrap(err)
				he.Exception = true
				return he
			}

			arr := make([]string, 0, len(data))
			for key, values := range data {
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
			body = []byte("{" + strings.Join(arr, ",") + "}")
		}
		c.RequestBody = body
		return c.Next()
	}
}
