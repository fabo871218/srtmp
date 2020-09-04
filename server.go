package srtmp

import (
	"errors"

	"github.com/fabo871218/srtmp/logger"
	"github.com/fabo871218/srtmp/protocol"
)

//ServeRtmp start rtmp server
func ServeRtmp(addr string, opts ...SettingFunc) error {
	if addr == "" {
		return errors.New("addr is not allowed to be empty")
	}

	setting := &SettingEngine{}
	for _, v := range opts {
		v(setting)
	}

	if setting.loggerFactory == nil {
		setting.loggerFactory = logger.NewDefaultFactory()
	}

	server := protocol.NewRtmpServer(protocol.NewRtmpStream(), nil, setting.loggerFactory.NewLogger("Info"))
	if setting.tlsEnabled {
		return server.ServeTLS(addr, setting.tlsCrt, setting.tlsKey)
	}
	return server.Serve(addr)
}
