package main

import (
	"context"
	_ "embed"
	"log/slog"
	"os"
	"os/signal"
	"sync"

	"github.com/covrom/apigw"
	"gopkg.in/yaml.v3"
)

//go:embed config.yaml
var cfgYaml []byte

func main() {
	l := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(l)

	gatewayConfig := &apigw.GatewayConfig{}

	err := yaml.Unmarshal(cfgYaml, &gatewayConfig)
	if err != nil {
		slog.Error("config error", "err", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)

	srv, err := apigw.NewServer(ctx, gatewayConfig)
	if err != nil {
		slog.Error("apigw.NewServer error", "err", err)
		cancel()
		os.Exit(1)
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Error(srv.ListenAndServe().Error())
	}()

	<-ctx.Done()

	srv.Shutdown(ctx)

	wg.Wait()

	cancel()
	os.Exit(1)
}
