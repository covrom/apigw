package apigw

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httputil"

	"github.com/google/uuid"
)

type requestID struct{}

func RecoverWrap(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := uuid.NewString()
		ctx := context.WithValue(r.Context(), requestID{}, reqID)
		r = r.WithContext(ctx)
		defer func() {
			r := recover()
			if r != nil {
				var err error
				switch t := r.(type) {
				case string:
					err = errors.New(t)
				case error:
					err = t
				default:
					err = errors.New("Unknown error")
				}
				slog.Error("panic", "requestId", reqID, "err", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}()

		if r.Method == http.MethodConnect {
			slog.Error("unexpected CONNECT method", "requestId", reqID)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed),
				http.StatusMethodNotAllowed)
			return
		}

		dumpRequest, err := httputil.DumpRequest(r, false)
		if err != nil {
			slog.Error("can't dump request", "requestId", reqID, "err", err)
		}

		slog.Info("Request", "requestId", reqID, "content", dumpRequest)

		h.ServeHTTP(w, r)
	})
}

func NewHandler(p *httputil.ReverseProxy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// reqID, _ := r.Context().Value(requestID{}).(string)
		p.ServeHTTP(w, r)
	}
}
