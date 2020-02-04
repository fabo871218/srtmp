package srtmp

import "github.com/fabo871218/srtmp/protocol"

var (
	DefaultHandler *protocol.StreamHandler
)

func init() {
	DefaultHandler = protocol.NewRtmpStream()
}

func ServeRtmp(addr string) error {
	server := protocol.NewRtmpServer(DefaultHandler, nil)
	return server.Serve(addr)
}

func ServeRtmpTLS(addr, tlsCrt, tlsKey string) error {
	server := protocol.NewRtmpServer(DefaultHandler, nil)
	return server.ServeTLS(addr, tlsCrt, tlsKey)
}
