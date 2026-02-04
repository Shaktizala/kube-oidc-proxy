package logging

import (
	"net/http"
	"strings"

	"github.com/Improwised/kube-oidc-proxy/pkg/logger"
	"go.uber.org/zap"
	"k8s.io/apiserver/pkg/authentication/user"
)

const (
	UserHeaderClientIPKey = "Remote-Client-IP"
)

// logs the request
func LogSuccessfulRequest(req *http.Request, inboundUser user.Info, outboundUser user.Info) {
	remoteAddr := req.RemoteAddr
	indexOfColon := strings.Index(remoteAddr, ":")
	if indexOfColon > 0 {
		remoteAddr = remoteAddr[0:indexOfColon]
	}

	xFwdFor := findXForwardedFor(req.Header, remoteAddr)

	fields := []zap.Field{
		zap.String("src_ip", remoteAddr),
		zap.String("x_forwarded_for", xFwdFor),
		zap.String("uri", req.RequestURI),
		zap.String("inbound_user", inboundUser.GetName()),
		zap.Strings("inbound_groups", inboundUser.GetGroups()),
		zap.Any("inbound_extra", inboundUser.GetExtra()),
	}

	if outboundUser != nil {
		fields = append(fields,
			zap.String("outbound_user", outboundUser.GetName()),
			zap.Strings("outbound_groups", outboundUser.GetGroups()),
			zap.String("outbound_uid", outboundUser.GetUID()),
			zap.Any("outbound_extra", outboundUser.GetExtra()),
		)
	}

	logger.Logger.Info("AuSuccess", fields...)
}

// determines if the x-forwarded-for header is present, if so remove
// the remoteaddr since it is repetitive
func findXForwardedFor(headers http.Header, remoteAddr string) string {
	xFwdFor := headers.Get("x-forwarded-for")
	// clean off remoteaddr from x-forwarded-for
	if xFwdFor != "" {

		newXFwdFor := ""
		oneFound := false
		xFwdForIps := strings.Split(xFwdFor, ",")

		for _, ip := range xFwdForIps {
			ip = strings.TrimSpace(ip)

			if ip != remoteAddr {
				newXFwdFor = newXFwdFor + ip + ", "
				oneFound = true
			}

		}

		if oneFound {
			newXFwdFor = newXFwdFor[0 : len(newXFwdFor)-2]
		}

		xFwdFor = newXFwdFor

	}

	return xFwdFor
}

// logs the failed request
func LogFailedRequest(req *http.Request) {
	remoteAddr := req.RemoteAddr
	indexOfColon := strings.Index(remoteAddr, ":")
	if indexOfColon > 0 {
		remoteAddr = remoteAddr[0:indexOfColon]
	}

	logger.Logger.Info("AuFail",
		zap.String("src_ip", remoteAddr),
		zap.String("x_forwarded_for", req.Header.Get("x-forwarded-for")),
		zap.String("uri", req.RequestURI))
}
