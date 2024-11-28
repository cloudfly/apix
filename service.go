package apix

import (
	"encoding/json"
	"fmt"
	"net/http"
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
	NotFoundHandler    http.Handler
}

const (
	rpcResponseWrapPrefix = `{"code":0,"data":`
	rpcResponseWrapSuffix = `}`
)

func New(opts ...ServiceOption) *Service {
	srv := &Service{}
	for _, opt := range opts {
		opt(srv)
	}
	srv.mux = http.NewServeMux()
	srv.grpc = newGRPCHandler(srv.grpcHeaderPatterns)
	return srv
}

func (srv *Service) ANY(path string, h any)    { srv.mux.Handle(path, toHTTPHandlerFunc(h)) }
func (srv *Service) GET(path string, h any)    { srv.mux.Handle("GET "+path, toHTTPHandlerFunc(h)) }
func (srv *Service) POST(path string, h any)   { srv.mux.Handle("POST "+path, toHTTPHandlerFunc(h)) }
func (srv *Service) PUT(path string, h any)    { srv.mux.Handle("PUT "+path, toHTTPHandlerFunc(h)) }
func (srv *Service) PATCH(path string, h any)  { srv.mux.Handle("PATCH "+path, toHTTPHandlerFunc(h)) }
func (srv *Service) DELETE(path string, h any) { srv.mux.Handle("DELETE "+path, toHTTPHandlerFunc(h)) }
func (srv *Service) TRACE(path string, h any)  { srv.mux.Handle("TRACE "+path, toHTTPHandlerFunc(h)) }
func (srv *Service) HEAD(path string, h any)   { srv.mux.Handle("HEAD "+path, toHTTPHandlerFunc(h)) }
func (srv *Service) OPTION(path string, h any) { srv.mux.Handle("OPTION "+path, toHTTPHandlerFunc(h)) }
func (srv *Service) CONNECT(path string, h any) {
	srv.mux.Handle("CONNECT "+path, toHTTPHandlerFunc(h))
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

func (srv *Service) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, srv)
}

type ServiceOption func(*Service)

// WithNotFoundHandler specifics a http handler for 404 case.
func WithNotFoundHandler(h http.Handler) ServiceOption {
	return func(srv *Service) {
		srv.grpc.notFoundHandler = h
	}
}

// UseGRPCHeaders extends the http headers whould to be forward to grpc service.
// By default, only headers with 'grpcgateway-' key prefix, and permanent HTTP header(as specified by the IANA, e.g: Accept, Cookie, Host) will be forward.
func UseGRPCHeaders(patterns []string) ServiceOption {
	return func(srv *Service) {
		srv.grpc.headerPatterns = patterns
	}
}

func toHTTPHandlerFunc(handler any) http.HandlerFunc {
	switch handler.(type) {
	case Handler:
	case HandlerCode:
	case http.Handler:
	default:
		panic("wrong type of handler, it should implements apix.Handler or apix.HandlerCode interface")
	}

	t := reflect.TypeOf(handler)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return func(w http.ResponseWriter, req *http.Request) {
		var (
			start  = time.Now()
			err    error
			ctx    = req.Context()
			v      = reflect.New(t).Interface()
			data   any
			status = 0
		)

		// access log
		defer func() {
			log.Ctx(ctx).Info().Err(err).Any("params", v).Dur("cost", time.Since(start)).Int("code", status).
				Str("method", req.Method).Str("path", req.URL.Path).Msg("HTTP request")
		}()

		// parse the parameters from request
		err = binding.New(nil).BindAndValidate(v, req, pathParams{req: req})
		if err != nil {
			responseJSON(w, 400, ResponseBody{
				Code:    400,
				Message: err.Error(),
			})
			return
		}

		// execute the handler
		switch h := v.(type) {
		case Handler:
			data, err = h.Execute(req)
		case HandlerCode:
			data, status, err = h.ExecuteCode(req)
		case http.Handler:
			// original http handler
			h.ServeHTTP(w, req)
			return
		}

		// response error
		if err != nil {
			if status == 0 {
				status = 1
			}
			responseJSON(w, 200, ResponseBody{
				Code:    status,
				Message: err.Error(),
			})
			return
		}

		// response data
		if data != nil {
			value := reflect.ValueOf(data)
			if value.Kind() == reflect.Slice && value.Len() == 0 {
				data = []struct{}{}
			}
			responseJSON(w, 200, ResponseBody{Code: 0, Data: data})
		}

		// response nothing
	}
}

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
	Execute(req *http.Request) (any, error)
}

// HandlerCode is similar with apix.Handler, but can customze the http status code in response by the second return value.
type HandlerCode interface {
	ExecuteCode(req *http.Request) (any, int, error)
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

func responseJSON(w http.ResponseWriter, code int, data any) {
	content, err := json.Marshal(data)
	if err != nil {
		content, _ = json.Marshal(ResponseBody{
			Code:    1,
			Message: err.Error(),
		})
	}
	if code <= 0 {
		code = 200
	}
	w.WriteHeader(code)
	w.Write(content)
}
