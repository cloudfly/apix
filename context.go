package apix

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"time"
)

var (
	bbp *ByteBufferPool = &ByteBufferPool{}
)

type CtxKeyType int

// ContextKey is the key that a Context returns itself for.

const (
	ContextRequestKey CtxKeyType = 0
	ContextKey                   = 1
)

// Context is the most important part of gin. It allows us to pass variables between middleware,
// manage the flow, validate the JSON of a request and render a JSON response for example.
type Context struct {
	Request  *http.Request
	Writer   http.ResponseWriter
	mu       sync.RWMutex
	Keys     map[string]any
	srv      *Service
	body     *ByteBuffer
	returned bool
}

func (c *Context) reset() {
	c.Request = nil
	c.Writer = nil
	c.Keys = nil
	c.srv = nil
	bbp.Put(c.body)
	c.body = nil
	c.returned = false
}

func (c *Context) Body() []byte {
	if c.body != nil {
		return c.body.B
	}
	c.body = bbp.Get()
	c.body.ReadFrom(c.Request.Body)
	return c.body.B
}

// Deadline returns that there is no deadline (ok==false) when c.Request has no Context.
func (c *Context) Deadline() (deadline time.Time, ok bool) {
	return c.Request.Context().Deadline()
}

// Done returns nil (chan which will wait forever) when c.Request has no Context.
func (c *Context) Done() <-chan struct{} {
	return c.Request.Context().Done()
}

// Err returns nil when c.Request has no Context.
func (c *Context) Err() error {
	return c.Request.Context().Err()
}

// Value returns the value associated with this context for key, or nil
// if no value is associated with key. Successive calls to Value with
// the same key returns the same result.
func (c *Context) Value(key any) any {
	if key == ContextRequestKey {
		return c.Request
	}
	if key == ContextKey {
		return c
	}
	if keyAsString, ok := key.(string); ok {
		if val, exists := c.Get(keyAsString); exists {
			return val
		}
	}
	return c.Request.Context().Value(key)
}

// Set is used to store a new key/value pair exclusively for this context.
// It also lazy initializes  c.Keys if it was not used previously.
func (c *Context) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Keys == nil {
		c.Keys = make(map[string]any)
	}

	c.Keys[key] = value
}

// Get returns the value for the given key, ie: (value, true).
// If the value does not exist it returns (nil, false)
func (c *Context) Get(key string) (value any, exists bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, exists = c.Keys[key]
	return
}

// Return write the result and code into ResponseWriter
func (c *Context) Return(status int, data any, marshaler func(any) ([]byte, error)) {
	if c.returned {
		// warn log
		return
	}
	var (
		content []byte
		err     error
	)
	if data != nil {
		content, err = marshaler(data)
		if err != nil {
			content, _ = json.Marshal(ResponseBody{
				Code:    1,
				Message: err.Error(),
			})
		}
	}
	if status <= 0 {
		status = 200
	}
	c.Writer.WriteHeader(status)
	c.Writer.Write(content)
	c.returned = true
}

func (c *Context) SetContentType(s string) {
	c.Writer.Header().Set("ContentType", s)
}

func (c *Context) JSON(status int, data any) {
	c.SetContentType("application/json")
	c.Return(status, data, json.Marshal)
}

func (c *Context) Text(status int, data any) {
	c.SetContentType("text/plain")
	c.Return(status, data, MarshalText)
}

func (c *Context) Fail(status int, err error) {
	if _, ok := err.(ResponseBody); !ok {
		err = ResponseBody{
			Code:    1,
			Message: err.Error(),
		}
	}
	c.Return(status, err, json.Marshal)
}

func (c *Context) Failf(status int, msg string, args ...any) {
	c.Fail(status, fmt.Errorf(msg, args...))
}

func MarshalText(data any) ([]byte, error) {
	switch v := data.(type) {
	case string:
		return []byte(v), nil
	case []byte:
		return v, nil
	}
	return nil, fmt.Errorf("type %s can not be marshared into text, string or []byte requried", reflect.TypeOf(data).Name())
}
