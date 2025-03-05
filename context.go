package apix

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/cloudfly/apix/bytespool"
)

var (
	bbp = &bytespool.ByteBufferPool{}
)

type ctxKeyType int

// ContextKey is the key that a Context returns itself for.

const (
	contextKey = 1
)

// Context is the most important part of gin. It allows us to pass variables between middleware,
// manage the flow, validate the JSON of a request and render a JSON response for example.
type Context struct {
	Request  *http.Request
	Writer   http.ResponseWriter
	mu       sync.RWMutex
	Keys     map[string]any
	srv      *Service
	body     *bytespool.ByteBuffer
	returned bool
}

// Ctx peek *apix.Context from the given context, it return nil if *apix.Context not exist
func Ctx(ctx context.Context) *Context {
	v := ctx.Value(contextKey)
	if v == nil {
		return nil
	}
	return v.(*Context)
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

// With add self into given ctx by context.WithValue, and return the new context
func (c *Context) With(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextKey, c)
}

// Body return the body bytes
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
	if key == contextKey {
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
	Return(c.Writer, status, data, marshaler)
	c.returned = true
}

func (c *Context) SetContentType(s string) {
	c.Writer.Header().Set("ContentType", s)
}

func (c *Context) ReturnJSON(status int, data any) {
	c.Return(status, data, json.Marshal)
}

func (c *Context) ReturnText(status int, data any) {
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
	err := ResponseBody{
		Code:    1,
		Message: fmt.Errorf(msg, args...).Error(),
	}
	c.Return(status, err, json.Marshal)
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
