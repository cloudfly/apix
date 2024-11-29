package apix

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
)

var (
	DefaultService *Service
)

func init() {
	DefaultService = New()
}

func ListenAndServe(addr string) error { return DefaultService.ListenAndServe(addr) }

func ANY(path string, h any)            { DefaultService.ANY(path, h) }
func GET(path string, h any)            { DefaultService.GET(path, h) }
func POST(path string, h any)           { DefaultService.POST(path, h) }
func PUT(path string, h any)            { DefaultService.PUT(path, h) }
func PATCH(path string, h any)          { DefaultService.PATCH(path, h) }
func DELETE(path string, h any)         { DefaultService.DELETE(path, h) }
func TRACE(path string, h any)          { DefaultService.TRACE(path, h) }
func HEAD(path string, h any)           { DefaultService.HEAD(path, h) }
func OPTION(path string, h any)         { DefaultService.OPTION(path, h) }
func CONNECT(path string, h any)        { DefaultService.CONNECT(path, h) }
func GRPCGatewayMux() *runtime.ServeMux { return DefaultService.GRPCGatewayMux() }
func GROUP(path string, middlewares ...Middleware) *Group {
	return DefaultService.GROUP(path, middlewares...)
}

// Return write the result and code into ResponseWriter
func Return(w http.ResponseWriter, status int, data any, marshaler func(any) ([]byte, error)) {
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
	w.WriteHeader(status)
	w.Write(content)
}

func ReturnJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("ContentType", "application/json")
	Return(w, status, data, json.Marshal)
}

func ReturnText(w http.ResponseWriter, status int, data any) {
	w.Header().Set("ContentType", "text/plain")
	Return(w, status, data, MarshalText)
}

func Fail(w http.ResponseWriter, status int, err error) {
	if _, ok := err.(ResponseBody); !ok {
		err = ResponseBody{
			Code:    1,
			Message: err.Error(),
		}
	}
	Return(w, status, err, json.Marshal)
}

func Failf(w http.ResponseWriter, status int, msg string, args ...any) {
	Fail(w, status, fmt.Errorf(msg, args...))
}
