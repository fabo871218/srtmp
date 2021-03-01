package srtmp

import (
	"errors"

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

	if setting.onVerify == nil {
		verify := func(url string) error {
			return errors.New("unset onVerify")
		}
		setting.onVerify = verify
	}

	api.logger = setting.loggerFactory.NewLogger(setting.logLevel)
	api.setting = setting
	return api
}

//ServeRtmp 创建一个rtmp服务，并监听响应的地址
func (api *RtmpAPI) ServeRtmp(addr string) error {
	handler := protocol.NewStreamHandler(api.logger)
	handler.OnVerify(api.setting.onVerify)
	server := &Server{
		handler: handler,
		logger:  api.logger,
	}
	return server.Serve(addr)
}

//ServeRtmpTLS 创建一个rtmp服务，并监听响应的地址
func (api *RtmpAPI) ServeRtmpTLS(addr, tlsKey, tlsCrt string) error {
	handler := protocol.NewStreamHandler(api.logger)
	handler.OnVerify(api.setting.onVerify)
	server := &Server{
		handler: handler,
		logger:  api.logger,
	}
	return server.ServeTLS(addr, tlsCrt, tlsKey)
}

//NewRtmpClient 创建一个rtmp客户端
func (api *RtmpAPI) NewRtmpClient() *RtmpClient {
	return newRtmpClient(api.logger)
}
