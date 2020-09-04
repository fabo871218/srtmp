package srtmp

import "github.com/fabo871218/srtmp/logger"

//SettingFunc ...
type SettingFunc func(*SettingEngine)

//SettingEngine ...
type SettingEngine struct {
	tlsEnabled    bool
	tlsCrt        string
	tlsKey        string
	loggerFactory logger.LoggerFactory
}

//WithTLSEnabled 是否启用tls连接
func WithTLSEnabled(v bool) SettingFunc {
	return func(setting *SettingEngine) {
		setting.tlsEnabled = v
	}
}

//WithTLSCrt 设置tls证书文件
func WithTLSCrt(v string) SettingFunc {
	return func(setting *SettingEngine) {
		setting.tlsCrt = v
	}
}

//WithTLSKey 设置tls证书密钥
func WithTLSKey(v string) SettingFunc {
	return func(setting *SettingEngine) {
		setting.tlsKey = v
	}
}

//WithLoggerFactory 设置日志创建类
func WithLoggerFactory(v logger.LoggerFactory) SettingFunc {
	return func(setting *SettingEngine) {
		setting.loggerFactory = v
	}
}
