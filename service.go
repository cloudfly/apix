package apix

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"reflect"
	"time"

	"github.com/bytedance/go-tagexpr/v2/binding"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/rs/zerolog/log"
)

type Service struct {
	mux                *http.ServeMux
	grpc               *grpcHandler
	grpcHeaderPatterns []string
	notFoundHandler    http.Handler
	middlewares        []Middleware
	marshaler          func(data any) ([]byte, error)
}

const (
	rpcResponseWrapPrefix = `{"code":0,"data":`
	rpcResponseWrapSuffix = `}`
)

func New(opts ...ServiceOption) *Service {
	srv := &Service{
		marshaler: json.Marshal,
		mux:       http.NewServeMux(),
	}
	for _, opt := range opts {
		opt(srv)
	}
	srv.grpc = newGRPCHandler(srv)
	return srv
}

func (srv *Service) ANY(path string, h any) {
	srv.mux.Handle(path, srv.generateHandlerFunc(h, srv.middlewares))
}
func (srv *Service) GET(path string, h any) {
	srv.mux.Handle("GET "+path, srv.generateHandlerFunc(h, srv.middlewares))
}
func (srv *Service) POST(path string, h any) {
	srv.mux.Handle("POST "+path, srv.generateHandlerFunc(h, srv.middlewares))
}
func (srv *Service) PUT(path string, h any) {
	srv.mux.Handle("PUT "+path, srv.generateHandlerFunc(h, srv.middlewares))
}
func (srv *Service) PATCH(path string, h any) {
	srv.mux.Handle("PATCH "+path, srv.generateHandlerFunc(h, srv.middlewares))
}
func (srv *Service) DELETE(path string, h any) {
	srv.mux.Handle("DELETE "+path, srv.generateHandlerFunc(h, srv.middlewares))
}
func (srv *Service) TRACE(path string, h any) {
	srv.mux.Handle("TRACE "+path, srv.generateHandlerFunc(h, srv.middlewares))
}
func (srv *Service) HEAD(path string, h any) {
	srv.mux.Handle("HEAD "+path, srv.generateHandlerFunc(h, srv.middlewares))
}
func (srv *Service) OPTION(path string, h any) {
	srv.mux.Handle("OPTION "+path, srv.generateHandlerFunc(h, srv.middlewares))
}
func (srv *Service) CONNECT(path string, h any) {
	srv.mux.Handle("CONNECT "+path, srv.generateHandlerFunc(h, srv.middlewares))
}

// GROUP create a api group with custom url prefix and middlewares, the middlewares only works on handlers registerd on this group
func (srv *Service) GROUP(path string, middlewares ...Middleware) *Group {
	return &Group{
		prefix:      path,
		srv:         srv,
		middlewares: append(append([]Middleware{}, srv.middlewares...), middlewares...),
	}
}

// GRPCGatewayMux return the grpcgateway servemux, use it to register grpc service
func (srv *Service) GRPCGatewayMux() *runtime.ServeMux {
	return srv.grpc.mux
}

// ServeHTTP implements the http.Handler interface
func (srv *Service) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// hijack not found status to grpc gateway handler
	notFoundHijack := statusHijack{
		targetCode:     http.StatusNotFound,
		ResponseWriter: w,
		req:            req,
		handler:        srv.grpc,
	}
	srv.mux.ServeHTTP(&notFoundHijack, req)
}

func (srv *Service) newCtx(w http.ResponseWriter, r *http.Request) *Context {
	return &Context{
		Request: r,
		Writer:  w,
		srv:     srv,
	}
}

func (srv *Service) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, srv)
}

type ServiceOption func(*Service)

// WithNotFoundHandler specifics a http handler for 404 case.
func WithNotFoundHandler(h http.Handler) ServiceOption {
	return func(srv *Service) {
		srv.notFoundHandler = h
	}
}

// WithMiddleware specifics middlewares for all the service handlers.
func WithMiddleware(middlewares ...Middleware) ServiceOption {
	return func(srv *Service) {
		srv.middlewares = middlewares
	}
}

func WithResponseMarshaler(marshaler func(any) ([]byte, error)) ServiceOption {
	return func(srv *Service) {
		srv.marshaler = marshaler
	}
}

// UseGRPCHeaders extends the http headers whould to be forward to grpc service.
// By default, only headers with 'grpcgateway-' key prefix, and permanent HTTP header(as specified by the IANA, e.g: Accept, Cookie, Host) will be forward.
func UseGRPCHeaders(patterns ...string) ServiceOption {
	return func(srv *Service) {
		srv.grpcHeaderPatterns = patterns
	}
}

// Group represents a api group
type Group struct {
	srv         *Service
	prefix      string
	middlewares []Middleware
}

func (g *Group) ANY(p string, h any) {
	g.srv.mux.Handle(path.Join(g.prefix, p), g.srv.generateHandlerFunc(h, g.middlewares))
}
func (g *Group) GET(p string, h any) {
	g.srv.mux.Handle("GET "+path.Join(g.prefix, p), g.srv.generateHandlerFunc(h, g.middlewares))
}
func (g *Group) POST(p string, h any) {
	g.srv.mux.Handle("POST "+path.Join(g.prefix, p), g.srv.generateHandlerFunc(h, g.middlewares))
}
func (g *Group) PUT(p string, h any) {
	g.srv.mux.Handle("PUT "+path.Join(g.prefix, p), g.srv.generateHandlerFunc(h, g.middlewares))
}
func (g *Group) PATCH(p string, h any) {
	g.srv.mux.Handle("PATCH "+path.Join(g.prefix, p), g.srv.generateHandlerFunc(h, g.middlewares))
}
func (g *Group) DELETE(p string, h any) {
	g.srv.mux.Handle("DELETE "+path.Join(g.prefix, p), g.srv.generateHandlerFunc(h, g.middlewares))
}
func (g *Group) TRACE(p string, h any) {
	g.srv.mux.Handle("TRACE "+path.Join(g.prefix, p), g.srv.generateHandlerFunc(h, g.middlewares))
}
func (g *Group) HEAD(p string, h any) {
	g.srv.mux.Handle("HEAD "+path.Join(g.prefix, p), g.srv.generateHandlerFunc(h, g.middlewares))
}
func (g *Group) OPTION(p string, h any) {
	g.srv.mux.Handle("OPTION "+path.Join(g.prefix, p), g.srv.generateHandlerFunc(h, g.middlewares))
}
func (g *Group) CONNECT(p string, h any) {
	g.srv.mux.Handle("CONNECT "+p, g.srv.generateHandlerFunc(h, g.middlewares))
}

// GROUP create a sub group base on this group. The url path and middlewares in arguments will append to the parent group's path and middlewares
func (g *Group) GROUP(p string, middlewares ...Middleware) *Group {
	return &Group{
		prefix:      path.Join(g.prefix, p),
		srv:         g.srv,
		middlewares: append(append([]Middleware{}, g.middlewares...), middlewares...),
	}
}

func (srv *Service) generateHandlerFunc(handler any, middlewares []Middleware) http.HandlerFunc {
	htype := 0
	switch h := handler.(type) {
	case Handler:
		htype = 1
	case HandlerCode:
		htype = 2
	case func(http.ResponseWriter, *http.Request):
		handler = http.HandlerFunc(h)
		htype = 3
	case http.HandlerFunc:
		htype = 3
	case http.Handler:
		htype = 4
	default:
		panic("wrong type of handler, it should implements apix.Handler, apix.HandlerCode or http.Handler interface")
	}

	t := reflect.TypeOf(handler)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	h := func(w http.ResponseWriter, r *http.Request) {
		var (
			start  = time.Now()
			err    error
			ctx    = srv.newCtx(w, r)
			v      = reflect.New(t).Interface()
			data   any
			status = 0
		)

		// access log
		defer func() {
			log.Ctx(ctx).Info().Err(err).Any("params", v).Dur("cost", time.Since(start)).Int("code", status).
				Str("method", r.Method).Str("path", r.URL.Path).Msg("HTTP request")
		}()

		if htype == 1 || htype == 2 {
			// parse the parameters from request
			if err = binding.New(nil).BindAndValidate(v, r, pathParams{req: r}); err != nil {
				ctx.ReturnJSON(400, ResponseBody{
					Code:    400,
					Message: err.Error(),
				})
				return
			}
		}

		// execute the handler
		switch htype {
		case 1:
			data, err = v.(Handler).Execute(ctx)
		case 2:
			data, status, err = v.(HandlerCode).ExecuteCode(ctx)
		case 3:
			// original http handler func
			handler.(http.HandlerFunc)(ctx.Writer, ctx.Request)
			return
		case 4:
			// original http handler
			handler.(http.Handler).ServeHTTP(ctx.Writer, ctx.Request)
			return
		}

		// response error
		if err != nil {
			if status == 0 {
				status = 1
			}
			ctx.ReturnJSON(200, ResponseBody{
				Code:    status,
				Message: err.Error(),
			})
			return
		}

		// response data
		if data != nil {
			if _, ok := data.(ResponseBody); ok {
				// data's type is ResponseBody, response directly
				ctx.ReturnJSON(200, data)
			} else {
				value := reflect.ValueOf(data)
				if value.Kind() == reflect.Slice && value.Len() == 0 {
					// return empty array instread of null for nil slice
					data = []struct{}{}
				}
				ctx.ReturnJSON(200, ResponseBody{Code: 0, Data: data})
			}
			return
		}

		// response nothing
		w.WriteHeader(http.StatusNoContent)
	}

	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// Middleware wrap the http.HandlerFunc, so that it can handle the http.Request in advance and intercept the request if required(eg. authorization, logging)
type Middleware func(http.HandlerFunc) http.HandlerFunc

// ResponseBody represents data type in response body
type ResponseBody struct {
	Code    int    `json:"code"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}

// Error implements the error interface, it return empty string if ResponseBody.Code == 0
func (reb ResponseBody) Error() string {
	if reb.Code == 0 {
		return ""
	}
	return fmt.Sprintf("%d: %s", reb.Code, reb.Message)
}

// Handler is a function type for handling http.Request, the return value will be marshaled into json before writing into response.
type Handler interface {
	Execute(*Context) (any, error)
}

// HandlerCode is similar with apix.Handler, but can customze the http status code in response by the second return value.
type HandlerCode interface {
	ExecuteCode(*Context) (any, int, error)
}

// pathParams implements the binding.PathParams interface for http.Request, so that the binding.BindAndValidate can parse the parameters in the request path.
type pathParams struct {
	req *http.Request
}

// Get the parameter in url path
//
// Note: the second return value always be true, it mainly used to satisfy the binding.PathParams interface
func (pp pathParams) Get(name string) (string, bool) {
	value := pp.req.PathValue(name)
	return value, true
}
