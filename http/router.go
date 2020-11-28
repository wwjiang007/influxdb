package http

import (
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"sync"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-stack/stack"
	"github.com/influxdata/httprouter"
	platform "github.com/influxdata/influxdb/v2"
	kithttp "github.com/influxdata/influxdb/v2/kit/transport/http"
	influxlogger "github.com/influxdata/influxdb/v2/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewRouter returns a new router with a 404 handler, a 405 handler, and a panic handler.
func NewRouter(h platform.HTTPErrorHandler) *httprouter.Router {
	b := baseHandler{HTTPErrorHandler: h}
	router := httprouter.New()
	router.NotFound = http.HandlerFunc(b.notFound)
	router.MethodNotAllowed = http.HandlerFunc(b.methodNotAllowed)
	router.PanicHandler = b.panic
	router.AddMatchedRouteToContext = true
	return router
}

// NewBaseChiRouter returns a new chi router with a 404 handler, a 405 handler, and a panic handler.
func NewBaseChiRouter(api *kithttp.API) chi.Router {
	router := chi.NewRouter()
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		api.Err(w, r, &platform.Error{
			Code: platform.ENotFound,
			Msg:  "path not found",
		})
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		api.Err(w, r, &platform.Error{
			Code: platform.EMethodNotAllowed,
			Msg:  fmt.Sprintf("allow: %s", w.Header().Get("Allow")),
		})

	})
	router.Use(
		panicMW(api),
		kithttp.SkipOptions,
		middleware.StripSlashes,
		kithttp.SetCORS,
	)
	return router
}

type baseHandler struct {
	platform.HTTPErrorHandler
}

// notFound represents a 404 handler that return a JSON response.
func (h baseHandler) notFound(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pe := &platform.Error{
		Code: platform.ENotFound,
		Msg:  "path not found",
	}

	h.HandleHTTPError(ctx, pe, w)
}

// methodNotAllowed represents a 405 handler that return a JSON response.
func (h baseHandler) methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	allow := w.Header().Get("Allow")
	pe := &platform.Error{
		Code: platform.EMethodNotAllowed,
		Msg:  fmt.Sprintf("allow: %s", allow),
	}

	h.HandleHTTPError(ctx, pe, w)
}

// panic handles panics recovered from http handlers.
// It returns a json response with http status code 500 and the recovered error message.
func (h baseHandler) panic(w http.ResponseWriter, r *http.Request, rcv interface{}) {
	ctx := r.Context()
	pe := &platform.Error{
		Code: platform.EInternal,
		Msg:  "a panic has occurred",
		Err:  fmt.Errorf("%s: %v", r.URL.String(), rcv),
	}

	l := getPanicLogger()
	if entry := l.Check(zapcore.ErrorLevel, pe.Msg); entry != nil {
		entry.Stack = string(debug.Stack())
		entry.Write(zap.Error(pe.Err))
	}

	h.HandleHTTPError(ctx, pe, w)
}

func panicMW(api *kithttp.API) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				panicErr := recover()
				if panicErr == nil {
					return
				}

				pe := &platform.Error{
					Code: platform.EInternal,
					Msg:  "a panic has occurred",
					Err:  fmt.Errorf("%s: %v", r.URL.String(), panicErr),
				}

				l := getPanicLogger()
				if entry := l.Check(zapcore.ErrorLevel, pe.Msg); entry != nil {
					entry.Stack = fmt.Sprintf("%+v", stack.Trace())
					entry.Write(zap.Error(pe.Err))
				}

				api.Err(w, r, pe)
			}()
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

var panicLogger *zap.Logger
var panicLoggerOnce sync.Once

// getPanicLogger returns a logger for panicHandler.
func getPanicLogger() *zap.Logger {
	panicLoggerOnce.Do(func() {
		panicLogger = influxlogger.New(os.Stderr)
		panicLogger = panicLogger.With(zap.String("handler", "panic"))
	})

	return panicLogger
}
