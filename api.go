package srtmp

import (
	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/container/flv"
	"github.com/fabo871218/srtmp/logger"
	"github.com/fabo871218/srtmp/protocol"
)

//RtmpAPI api接口类
type RtmpAPI struct {
	setting *SettingEngine
	logger  logger.Logger
}

//NewAPI 创建一个api，设置相应的参数信息
func NewAPI(opts ...SettingFunc) *RtmpAPI {
	api := &RtmpAPI{}

	setting := &SettingEngine{}
	for _, v := range opts {
		v(setting)
	}

	if setting.loggerFactory == nil {
		setting.loggerFactory = logger.NewDefaultFactory()
	}

	if setting.logLevel == logger.LogLevelDisabled {
		setting.logLevel = logger.LogLevelInfo
	}
	api.logger = setting.loggerFactory.NewLogger(setting.logLevel)
	api.setting = setting
	return api
}

//ServeRtmp 创建一个rtmp服务，并监听响应的地址
func (api *RtmpAPI) ServeRtmp(addr string) error {
	server := &Server{
		handler: protocol.NewStreamHandler(api.logger),
		logger:  api.logger,
	}
	return server.Serve(addr)
}

//ServeRtmpTLS 创建一个rtmp服务，并监听响应的地址
func (api *RtmpAPI) ServeRtmpTLS(addr, tlsKey, tlsCrt string) error {
	server := &Server{
		handler: protocol.NewStreamHandler(api.logger),
		logger:  api.logger,
	}
	return server.ServeTLS(addr, tlsKey, tlsCrt)
}

//NewRtmpClient 创建一个rtmp客户端
func (api *RtmpAPI) NewRtmpClient() *RtmpClient {
	client := &RtmpClient{
		packetChan: make(chan *av.Packet, 16),
		videoFirst: true,
		audioFirst: true,
		demuxer:    flv.NewDemuxer(),
		logger:     api.logger,
	}
	return client
}
