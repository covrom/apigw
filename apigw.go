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
	ListenAddr string  `yaml:"listen"`
	Specs      []Route `yaml:"specs"`
}

type Route struct {
	// An array of Server Objects, which provide connectivity information to a target server.
	// If the servers property is not provided, or is an empty array,
	// the default value would be a Server Object with a url value of /.
	Servers []Server `yaml:"servers"`

	// The available paths and operations for the API.
	Paths map[string]PathItem `yaml:"paths"`
}

type Server struct {
	// A URL to the target host. This URL supports Server Variables and MAY be relative,
	// to indicate that the host location is relative to the location where the OpenAPI
	// document is being served. Variable substitutions will be made when a variable
	// is named in {brackets}.
	Url string `yaml:"url"`
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

type Operation struct {
	OperationID string `yaml:"operationId"`
	// Responses   Responses
}

func NewServer(ctx context.Context, gatewayConfig *GatewayConfig) (*http.Server, error) {
	slog.Info("Initializing routes...")

	r := http.NewServeMux()

	for _, route := range gatewayConfig.Specs {
		proxy, err := NewProxy(route.Target)
		if err != nil {
			return nil, fmt.Errorf("NewProxy error for target %q: %w", route.Target, err)
		}

		slog.Info(fmt.Sprintf("Mapping %q", route.Name),
			"context", route.Context, "target", route.Target)

		pth, err := url.JoinPath(route.Context, "{targetPath...}")
		if err != nil {
			return nil, fmt.Errorf("JoinPath for context %q error: %w", route.Context, err)
		}

		r.Handle(pth, RecoverWrap(NewHandler(proxy)))
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

	return srv, nil
}
