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
	defaultRequestBodyLimit     = 50 * 1024
	jsonContentType             = "application/json"
	formURLEneltonedContentType = "application/x-www-form-urleneltoned"
)

type (
	// Deeltone body deeltone function
	Deeltone func(c *elton.Context, originalData []byte) (data []byte, err error)
	// Config json parser config
	Config struct {
		// Limit the limit size of body
		Limit int
		// IgnoreJSON ignore json type
		IgnoreJSON bool
		// IgnoreFormURLEneltoned ignore form url eneltoned type
		IgnoreFormURLEneltoned bool
		// Deeltone deeltone function
		Deeltone Deeltone
		Skipper  elton.Skipper
	}
)

var (
	validMethods = []string{
		http.MethodPost,
		http.MethodPatch,
		http.MethodPut,
	}
)

// DefaultDeeltone default deeltone
func DefaultDeeltone(c *elton.Context, data []byte) ([]byte, error) {
	eneltoning := c.GetRequestHeader(elton.HeaderContentEncoding)
	if eneltoning == elton.Gzip {
		c.SetRequestHeader(elton.HeaderContentEncoding, "")
		return doGunzip(data)
	}
	return data, nil
}

// NewDefault create a default body parser, default limit and only json parser
func NewDefault() elton.Handler {
	return New(Config{
		IgnoreFormURLEneltoned: true,
		Deeltone:               DefaultDeeltone,
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
		isFormURLEneltoned := ctFields[0] == formURLEneltonedContentType

		// 如果不是 json 也不是 form url eneltoned，则跳过
		if !isJSON && !isFormURLEneltoned {
			return c.Next()
		}
		// 如果数据类型json，而且中间件不处理，则跳过
		if isJSON && config.IgnoreJSON {
			return c.Next()
		}

		// 如果数据类型form url eneltoned，而且中间件不处理，则跳过
		if isFormURLEneltoned && config.IgnoreFormURLEneltoned {
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
		if config.Deeltone != nil {
			// 对提交数据做deeltone处理（如编码转换等）
			body, err = config.Deeltone(c, body)
			if err != nil {
				return
			}
		}
		// 将form url eneltoned 数据转化为json
		if isFormURLEneltoned {
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
