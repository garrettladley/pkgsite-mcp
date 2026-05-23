package middleware

import (
	"net/http"

	"github.com/garrettladley/pkgsite-mcp/internal/observability"
)

func Recovery(obs *observability.Handle) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					obs.Recover(r.Context(), recovered)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
