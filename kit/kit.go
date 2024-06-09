package kit

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"github.com/a-h/templ"
)

type HandlerFunc func(kit *Kit) error

type Authenticater interface {
	Authenticate(http.ResponseWriter, *http.Request) error
}

type ErrorHandlerFunc func(kit *Kit, err error)

type AuthKey struct{}

type Auth interface {
	Check() bool
}

var (
	errorHandler = func(kit *Kit, err error) {
		kit.Text(http.StatusInternalServerError, err.Error())
	}
)

type DefaultAuth struct{}

func (DefaultAuth) Check() bool { return false }

type Kit struct {
	Response http.ResponseWriter
	Request  *http.Request
}

func UseErrorHandler(h ErrorHandlerFunc) { errorHandler = h }

func (kit *Kit) Auth() Auth {
	value, ok := kit.Request.Context().Value(AuthKey{}).(Auth)
	if !ok {
		slog.Warn("kit authentication not set")
		return DefaultAuth{}
	}
	return value
}

// Redirect with HTMX support.
func (kit *Kit) Redirect(status int, url string) error {
	if len(kit.Request.Header.Get("HX-Request")) > 0 {
		kit.Response.Header().Set("HX-Redirect", url)
		kit.Response.WriteHeader(http.StatusSeeOther)
		return nil
	}
	return kit.Redirect(status, url)
}

func (kit *Kit) JSON(status int, v any) error {
	kit.Response.WriteHeader(status)
	kit.Response.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(kit.Response).Encode(v)
}

func (kit *Kit) Text(status int, msg string) error {
	kit.Response.WriteHeader(status)
	kit.Response.Header().Set("Content-Type", "text/plain")
	_, err := kit.Response.Write([]byte(msg))
	return err
}

func (kit *Kit) Bytes(status int, b []byte) error {
	kit.Response.WriteHeader(status)
	kit.Response.Header().Set("Content-Type", "text/plain")
	_, err := kit.Response.Write(b)
	return err
}

func (kit *Kit) Render(c templ.Component) error {
	return c.Render(kit.Request.Context(), kit.Response)
}

func Handler(h HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kit := &Kit{
			Response: w,
			Request:  r,
		}
		if err := h(kit); err != nil {
			if errorHandler != nil {
				errorHandler(kit, err)
				return
			}
			kit.Text(http.StatusInternalServerError, err.Error())
		}
	}
}

type AuthenticationConfig struct {
	AuthFunc    func(http.ResponseWriter, *http.Request) (Auth, error)
	RedirectURL string
}

func WithAuthentication(config AuthenticationConfig, strict bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			kit := &Kit{
				Response: w,
				Request:  r,
			}
			auth, err := config.AuthFunc(w, r)
			if err != nil {
				errorHandler(kit, err)
				return
			}
			if strict && !auth.Check() && r.URL.Path != config.RedirectURL {
				kit.Redirect(http.StatusSeeOther, config.RedirectURL)
				return
			}

			ctx := context.WithValue(r.Context(), AuthKey{}, auth)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func IsDevelopment() bool {
	return os.Getenv("APP_ENV") == "development"
}

func IsProduction() bool {
	return os.Getenv("APP_ENV") == "production"
}

func Env() string {
	return os.Getenv("APP_ENV")
}
