package bodyparser

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vicanso/elton"
	"github.com/vicanso/hes"
)

type (
	errReadCloser struct {
		customErr error
	}
)

// Read read function
func (er *errReadCloser) Read(p []byte) (n int, err error) {
	return 0, er.customErr
}

// Close close function
func (er *errReadCloser) Close() error {
	return nil
}

// NewErrorReadCloser create an read error
func NewErrorReadCloser(err error) io.ReadCloser {
	r := &errReadCloser{
		customErr: err,
	}
	return r
}

func TestGzipDecoder(t *testing.T) {
	gzipDecoder := NewGzipDecoder()
	assert := assert.New(t)
	originalBuf := []byte("abcdabcdabcd")
	var b bytes.Buffer
	w, _ := gzip.NewWriterLevel(&b, 9)
	_, err := w.Write(originalBuf)
	assert.Nil(err)
	w.Close()

	c := elton.NewContext(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	assert.False(gzipDecoder.Validate(c))

	c.SetRequestHeader(elton.HeaderContentEncoding, elton.Gzip)
	assert.True(gzipDecoder.Validate(c))
	buf, err := gzipDecoder.Decode(c, b.Bytes())
	assert.Nil(err)
	assert.Equal(originalBuf, buf)

	_, err = gzipDecoder.Decode(c, []byte("ab"))
	assert.NotNil(err)
}

func TestJSONDecoder(t *testing.T) {
	assert := assert.New(t)
	jsonDecoder := NewJSONDecoder()
	c := elton.NewContext(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	assert.False(jsonDecoder.Validate(c))
	c.SetRequestHeader(elton.HeaderContentType, elton.MIMEApplicationJSON)
	assert.True(jsonDecoder.Validate(c))

	buf := []byte(`{"a": 1}`)
	data, err := jsonDecoder.Decode(c, buf)
	assert.Nil(err)
	assert.Equal(buf, data)
	_, err = jsonDecoder.Decode(c, []byte("abcd"))
	assert.Equal(errInvalidJSON, err)

	_, err = jsonDecoder.Decode(c, []byte("{abcd"))
	assert.Equal(errInvalidJSON, err)

	_, err = jsonDecoder.Decode(c, []byte("[abcd"))
	assert.Equal(errInvalidJSON, err)
}

func TestFormURLEncodedDecoder(t *testing.T) {
	assert := assert.New(t)
	formURLEncodedDecoder := NewFormURLEncodedDecoder()
	c := elton.NewContext(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	assert.False(formURLEncodedDecoder.Validate(c))
	c.SetRequestHeader(elton.HeaderContentType, "application/x-www-form-urlencoded; charset=UTF-8")
	assert.True(formURLEncodedDecoder.Validate(c))

	data, err := formURLEncodedDecoder.Decode(c, []byte("a=1&b=2"))
	assert.Nil(err)
	assert.Equal(17, len(data))
}

func TestBodyParser(t *testing.T) {
	t.Run("skip", func(t *testing.T) {
		assert := assert.New(t)
		bodyParser := New(Config{
			Skipper: func(c *elton.Context) bool {
				return true
			},
		})

		body := `{"name": "tree.xie"}`
		req := httptest.NewRequest("POST", "https://aslant.site/", strings.NewReader(body))
		req.Header.Set(elton.HeaderContentType, "application/json")
		c := elton.NewContext(nil, req)
		done := false
		c.Next = func() error {
			done = true
			return nil
		}
		err := bodyParser(c)
		assert.Nil(err)
		assert.True(done)
		assert.Equal(len(c.RequestBody), 0)
	})

	t.Run("request body is not nil", func(t *testing.T) {
		assert := assert.New(t)
		bodyParser := NewDefault()

		body := `{"name": "tree.xie"}`
		req := httptest.NewRequest("POST", "https://aslant.site/", strings.NewReader(body))
		req.Header.Set(elton.HeaderContentType, "application/json")
		c := elton.NewContext(nil, req)
		done := false
		c.Next = func() error {
			done = true
			return nil
		}
		c.RequestBody = []byte("a")
		err := bodyParser(c)

		assert.Nil(err)
		assert.True(done)
		assert.Equal(c.RequestBody, []byte("a"))
	})

	t.Run("pass method", func(t *testing.T) {
		assert := assert.New(t)
		bodyParser := New(Config{})
		req := httptest.NewRequest("GET", "https://aslant.site/", nil)
		c := elton.NewContext(nil, req)
		done := false
		c.Next = func() error {
			done = true
			return nil
		}
		err := bodyParser(c)
		assert.Nil(err)
		assert.True(done)
	})

	t.Run("pass content type not json", func(t *testing.T) {
		assert := assert.New(t)
		bodyParser := New(Config{})
		req := httptest.NewRequest("POST", "https://aslant.site/", strings.NewReader("abc"))
		c := elton.NewContext(nil, req)
		done := false
		c.Next = func() error {
			done = true
			return nil
		}
		err := bodyParser(c)
		assert.Nil(err)
		assert.True(done)
	})

	t.Run("read body fail", func(t *testing.T) {
		assert := assert.New(t)
		bodyParser := New(Config{})
		req := httptest.NewRequest("POST", "https://aslant.site/", NewErrorReadCloser(hes.New("abc")))
		req.Header.Set(elton.HeaderContentType, "application/json")
		c := elton.NewContext(nil, req)
		err := bodyParser(c)
		assert.NotNil(err)
		assert.Equal(err.Error(), "category=elton-body-parser, message=message=abc")
	})

	t.Run("body over limit size", func(t *testing.T) {
		assert := assert.New(t)
		bodyParser := New(Config{
			Limit: 1,
		})
		req := httptest.NewRequest("POST", "https://aslant.site/", strings.NewReader("abc"))
		req.Header.Set(elton.HeaderContentType, "application/json")
		c := elton.NewContext(nil, req)
		err := bodyParser(c)
		assert.NotNil(err)
		assert.Equal(err.Error(), "category=elton-body-parser, message=request body is 3 bytes, it should be <= 1")
	})

	t.Run("parse json success", func(t *testing.T) {
		assert := assert.New(t)
		conf := Config{}
		conf.AddDecoder(NewJSONDecoder())
		bodyParser := New(conf)
		body := `{"name": "tree.xie"}`
		req := httptest.NewRequest("POST", "https://aslant.site/", strings.NewReader(body))
		req.Header.Set(elton.HeaderContentType, "application/json")
		c := elton.NewContext(nil, req)
		done := false
		c.Next = func() error {
			done = true
			if string(c.RequestBody) != body {
				return hes.New("request body is invalid")
			}
			return nil
		}
		err := bodyParser(c)
		assert.Nil(err)
		assert.True(done)
	})

	t.Run("parse json(gzip) success", func(t *testing.T) {
		assert := assert.New(t)
		conf := Config{}
		conf.AddDecoder(NewGzipDecoder())
		conf.AddDecoder(NewJSONDecoder())
		bodyParser := New(conf)
		originalBuf := []byte(`{"name": "tree.xie"}`)
		var b bytes.Buffer
		w, _ := gzip.NewWriterLevel(&b, 9)
		_, err := w.Write(originalBuf)
		assert.Nil(err)
		w.Close()

		req := httptest.NewRequest("POST", "https://aslant.site/", bytes.NewReader(b.Bytes()))
		req.Header.Set(elton.HeaderContentType, "application/json")
		req.Header.Set(elton.HeaderContentEncoding, "gzip")
		c := elton.NewContext(nil, req)
		done := false
		c.Next = func() error {
			done = true
			if !bytes.Equal(c.RequestBody, originalBuf) {
				return hes.New("request body is invalid")
			}
			return nil
		}
		err = bodyParser(c)
		assert.Nil(err)
		assert.True(done)
	})

	t.Run("decode data success", func(t *testing.T) {
		assert := assert.New(t)
		conf := Config{}
		conf.AddDecoder(&Decoder{
			Validate: func(c *elton.Context) bool {
				return c.GetRequestHeader(elton.HeaderContentType) == "application/json;charset=base64"
			},
			Decode: func(c *elton.Context, originalData []byte) (data []byte, err error) {
				return base64.RawStdEncoding.DecodeString(string(originalData))
			},
		})

		bodyParser := New(conf)
		body := `{"name": "tree.xie"}`
		b64 := base64.RawStdEncoding.EncodeToString([]byte(body))
		req := httptest.NewRequest("POST", "https://aslant.site/", strings.NewReader(b64))
		req.Header.Set(elton.HeaderContentType, "application/json;charset=base64")
		c := elton.NewContext(nil, req)
		done := false
		c.Next = func() error {
			done = true
			if string(c.RequestBody) != body {
				return hes.New("request body is invalid")
			}
			return nil
		}
		err := bodyParser(c)
		assert.Nil(err)
		assert.True(done)
	})

	t.Run("parse form url encoded success", func(t *testing.T) {
		assert := assert.New(t)
		conf := Config{}
		conf.AddDecoder(NewFormURLEncodedDecoder())
		bodyParser := New(conf)
		body := `name=tree.xie&type=1&type=2`
		req := httptest.NewRequest("POST", "https://aslant.site/", strings.NewReader(body))
		req.Header.Set(elton.HeaderContentType, "application/x-www-form-urlencoded")
		c := elton.NewContext(nil, req)
		done := false
		c.Next = func() error {
			done = true
			if len(c.RequestBody) != 36 {
				return hes.New("request body is invalid")
			}
			return nil
		}
		err := bodyParser(c)
		assert.Nil(err)
		assert.True(done)
	})
}

// https://stackoverflow.com/questions/50120427/fail-unit-tests-if-coverage-is-below-certain-percentage
func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	rc := m.Run()

	// rc 0 means we've passed,
	// and CoverMode will be non empty if run with -cover
	if rc == 0 && testing.CoverMode() != "" {
		c := testing.Coverage()
		if c < 0.9 {
			fmt.Println("Tests passed but coverage failed at", c)
			rc = -1
		}
	}
	os.Exit(rc)
}
