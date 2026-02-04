// Copyright Jetstack Ltd. See LICENSE for details.
package probe

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Improwised/kube-oidc-proxy/pkg/logger"
	"github.com/heptiolabs/healthcheck"
	"go.uber.org/zap"
	"k8s.io/apiserver/pkg/authentication/authenticator"
)

const (
	timeout = time.Second * 10
)

type HealthCheck struct {
	handler healthcheck.Handler

	oidcAuther authenticator.Token
	fakeJWT    string

	ready bool
}

func Run(port, fakeJWT string, oidcAuther authenticator.Token) error {
	h := &HealthCheck{
		handler:    healthcheck.NewHandler(),
		oidcAuther: oidcAuther,
		fakeJWT:    fakeJWT,
	}

	h.handler.AddReadinessCheck("secure serving", h.Check)

	go func() {
		for {
			err := http.ListenAndServe(net.JoinHostPort("0.0.0.0", port), h.handler)
			if err != nil {
				logger.Logger.Error("ready probe listener failed", zap.Error(err))
			}
			time.Sleep(5 * time.Second)
		}
	}()

	return nil
}

func (h *HealthCheck) Check() error {
	if h.ready {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, _, err := h.oidcAuther.AuthenticateToken(ctx, h.fakeJWT)
	if err != nil && strings.HasSuffix(err.Error(), "authenticator not initialized") {
		err = fmt.Errorf("OIDC provider not yet initialized: %s", err)
		logger.Logger.Debug("OIDC provider not yet initialized", zap.Error(err))
		return err
	}

	h.ready = true

	logger.Logger.Debug("OIDC provider initialized")

	return nil
}
