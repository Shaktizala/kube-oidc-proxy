// Copyright Jetstack Ltd. See LICENSE for details.
package util

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/Improwised/kube-oidc-proxy/pkg/logger"
	"go.uber.org/zap"
)

func SignalHandler() chan struct{} {
	stopCh := make(chan struct{})
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-ch

		close(stopCh)

		for i := 0; i < 3; i++ {
			logger.Logger.Info("received signal, shutting down gracefully", zap.String("signal", sig.String()))
			sig = <-ch
		}

		logger.Logger.Info("received signal, force closing", zap.String("signal", sig.String()))

		os.Exit(1)
	}()

	return stopCh
}
