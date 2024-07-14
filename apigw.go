package apigw

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"
)

type GatewayConfig struct {
	ListenAddr        string        `yaml:"listen"`
	ReadHeaderTimeout time.Duration `yaml:"readHeaderTimeout"` // default 2 min
	ReadTimeout       time.Duration `yaml:"readTimeout"`       // default 2 min
	WriteTimeout      time.Duration `yaml:"writeTimeout"`      // default 2 min
	MaxHeaderBytes    int           `yaml:"maxHeaderBytes"`    // default 1 Mb
	Specs             []Route       `yaml:"specs"`
}

type Route struct {
	// URL for incoming request with or without domain name (host)
	Url string `yaml:"url"`
	// A URL with host (required) to the target host
	Target string `yaml:"target"`

	// The available paths and operations for the API.
	// Key is a relative path to an individual endpoint.
	// Key MUST begin with a forward slash (/).
	// The path is appended (no relative URL resolution)
	// to the expanded URL from the Url field in order to construct the full source URL
	// and from the Target field in order to construct the full tareget URL
	// Key patterns can match the path of a request. Some examples:
	// 	"/index.html" matches the path "/index.html" for any host and method.
	// 	"/static/" matches a request whose path begins with "/static/".
	// 	"/abc/{$}" matches requests with path "/abc/". The special wildcard {$} matches only the end of the URL.
	// 	"/b/{bucket}/o/{objectname...}" matches paths whose first segment is "b"
	//		and whose third segment is "o". The name "bucket" denotes the second segment and "objectname" denotes the remainder of the path.
	Paths map[string]PathItem `yaml:"paths"`
}

type PathItem struct {
	Get     *Operation `yaml:"get"`
	Put     *Operation `yaml:"put"`
	Post    *Operation `yaml:"post"`
	Delete  *Operation `yaml:"delete"`
	Options *Operation `yaml:"options"`
	Head    *Operation `yaml:"head"`
	Patch   *Operation `yaml:"patch"`
	Trace   *Operation `yaml:"trace"`
}

func (pi *PathItem) WalkMethods(f func(method string)) {
	if pi.Get != nil {
		f(http.MethodGet)
	}
	if pi.Put != nil {
		f(http.MethodPut)
	}
	if pi.Post != nil {
		f(http.MethodPost)
	}
	if pi.Delete != nil {
		f(http.MethodDelete)
	}
	if pi.Options != nil {
		f(http.MethodOptions)
	}
	if pi.Head != nil {
		f(http.MethodHead)
	}
	if pi.Patch != nil {
		f(http.MethodPatch)
	}
	if pi.Trace != nil {
		f(http.MethodTrace)
	}
}

type Operation struct{}

func NewServer(ctx context.Context, gatewayConfig *GatewayConfig) (*http.Server, error) {
	slog.Info("Initializing routes...")

	r := http.NewServeMux()

	for _, spec := range gatewayConfig.Specs {
		specUrl, err := url.Parse(spec.Url)
		if err != nil {
			return nil, fmt.Errorf("url.Parse error for spec.Url %q: %w", spec.Url, err)
		}
		specTarget, err := url.Parse(spec.Target)
		if err != nil {
			return nil, fmt.Errorf("url.Parse error for spec.Target %q: %w", spec.Target, err)
		}

		proxy, err := NewProxy(specTarget)
		if err != nil {
			return nil, fmt.Errorf("NewProxy error for target %q: %w", spec.Target, err)
		}

		slog.Info("Mapping", "url", spec.Url, "target", spec.Target)

		for p, pi := range spec.Paths {
			pth, err := url.JoinPath(specUrl.Path, p)
			if err != nil {
				return nil, fmt.Errorf("url.JoinPath for %q and %q error: %w", specUrl.Path, p, err)
			}
			if host := specUrl.Hostname(); host != "" {
				pth = fmt.Sprintf("%s/%s", host, pth)
			}
			pi.WalkMethods(func(m string) {
				mpth := fmt.Sprintf("%s %s", m, pth)
				slog.Info("Handle", "path", mpth)
				r.Handle(mpth, RecoverWrap(http.StripPrefix(specUrl.Path, NewHandler(proxy))))
			})
		}
	}

	slog.Info("Started server", "addr", gatewayConfig.ListenAddr)

	hlog := slog.NewLogLogger(slog.Default().Handler(), slog.LevelError)

	srv := &http.Server{
		Addr:              gatewayConfig.ListenAddr,
		Handler:           r,
		ErrorLog:          hlog,
		ReadHeaderTimeout: 2 * time.Minute,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      2 * time.Minute,
		MaxHeaderBytes:    1 << 20,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}

	if gatewayConfig.ReadHeaderTimeout > 0 {
		srv.ReadHeaderTimeout = gatewayConfig.ReadHeaderTimeout
	}
	if gatewayConfig.ReadTimeout > 0 {
		srv.ReadTimeout = gatewayConfig.ReadTimeout
	}
	if gatewayConfig.WriteTimeout > 0 {
		srv.WriteTimeout = gatewayConfig.WriteTimeout
	}
	if gatewayConfig.MaxHeaderBytes > 0 {
		srv.MaxHeaderBytes = gatewayConfig.MaxHeaderBytes
	}

	return srv, nil
}
