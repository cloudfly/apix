package apix

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

type statusHijack struct {
	http.ResponseWriter
	req        *http.Request
	handler    http.Handler
	targetCode int
	hijacked   bool
	served     bool
}

func (h *statusHijack) WriteHeader(code int) {
	if code == h.targetCode {
		// hijack it
		h.hijacked = true
		return
	}
	h.WriteHeader(code)
}

func (h *statusHijack) Write(body []byte) (int, error) {
	if h.hijacked {
		if !h.served {
			h.handler.ServeHTTP(h.ResponseWriter, h.req)
			h.served = true
		}
		return len(body), nil
	}
	return h.Write(body)
}

type grpcHandler struct {
	mux             *runtime.ServeMux
	notFoundHandler http.Handler
	headerPatterns  []string
}

func newGRPCHandler(headerPatterns []string) *grpcHandler {
	gh := &grpcHandler{
		mux: runtime.NewServeMux(
			runtime.WithMarshalerOption(
				runtime.MIMEWildcard, &runtime.JSONBuiltin{},
			),
			runtime.SetQueryParameterParser(&queryParser{}),
			runtime.WithIncomingHeaderMatcher(grpcHeaderMatcher(headerPatterns)),
			runtime.WithErrorHandler(func(ctx context.Context, mux *runtime.ServeMux, _ runtime.Marshaler, w http.ResponseWriter, r *http.Request, err error) {
				data := ResponseBody{
					Code:    1,
					Message: err.Error(),
				}
				log.Ctx(ctx).Error().Err(err).Str("method", r.Method).Str("path", r.RequestURI).Msg("Handling rpc request error")
				content, _ := json.Marshal(data)
				w.WriteHeader(200)
				w.Write(content)
			}),
			runtime.WithForwardResponseOption(func(ctx context.Context, w http.ResponseWriter, msg proto.Message) error {
				w.WriteHeader(200)
				w.Write([]byte(rpcResponseWrapPrefix))
				return nil
			}),
		),
	}
	return gh
}

func (gh *grpcHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if gh.notFoundHandler != nil {
		w = &statusHijack{
			ResponseWriter: w,
			req:            r,
			targetCode:     http.StatusNotFound,
			handler:        gh.notFoundHandler,
		}
	}
	gh.mux.ServeHTTP(w, r)
	w.Write([]byte(rpcResponseWrapSuffix))
}

func grpcHeaderMatcher(patterns []string) runtime.HeaderMatcherFunc {
	return func(key string) (string, bool) {
		for _, prefix := range patterns {
			if matchStr(prefix, key) {
				return key, true
			}
		}
		return runtime.DefaultHeaderMatcher(key)
	}
}

// matchStr match a string value with wildcard pattern
func matchStr(pattern, s string) bool {
	i, j, star, match := 0, 0, -1, 0
	for i < len(s) {
		if j < len(pattern) && (s[i] == pattern[j] || pattern[j] == '?') {
			i++
			j++
		} else if j < len(pattern) && pattern[j] == '*' {
			match, star = i, j
			j++
		} else if star != -1 {
			j = star + 1
			match++
			i = match
		} else {
			return false
		}
	}
	for ; j < len(pattern); j++ {
		if pattern[j] != '*' {
			return false
		}
	}
	return true
}
