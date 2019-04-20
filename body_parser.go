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
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/vicanso/cod"
	"github.com/vicanso/hes"
)

const (
	// ErrCategory body parser error category
	ErrCategory = "cod-body-parser"
	// 默认为50kb
	defaultRequestBodyLimit   = 50 * 1024
	jsonContentType           = "application/json"
	formURLEncodedContentType = "application/x-www-form-urlencoded"
)

type (
	// Config json parser config
	Config struct {
		// Limit the limit size of body
		Limit int
		// IgnoreJSON ignore json type
		IgnoreJSON bool
		// IgnoreFormURLEncoded ignore form url encoded type
		IgnoreFormURLEncoded bool
		Skipper              cod.Skipper
	}
)

var (
	validMethods = []string{
		http.MethodPost,
		http.MethodPatch,
		http.MethodPut,
	}
)

// NewDefault create a default body parser, default limit and only json parser
func NewDefault() cod.Handler {
	return New(Config{
		IgnoreFormURLEncoded: true,
	})
}

// New create a body parser
func New(config Config) cod.Handler {
	limit := defaultRequestBodyLimit
	if config.Limit != 0 {
		limit = config.Limit
	}
	skipper := config.Skipper
	if skipper == nil {
		skipper = cod.DefaultSkipper
	}
	return func(c *cod.Context) (err error) {
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
		ct := c.GetRequestHeader(cod.HeaderContentType)
		// 非json则跳过
		isJSON := strings.HasPrefix(ct, jsonContentType)
		isFormURLEncoded := strings.HasPrefix(ct, formURLEncodedContentType)

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
