package srtmp

import "github.com/fabo871218/srtmp/logger"

//SettingFunc ...
type SettingFunc func(*SettingEngine)

//SettingEngine ...
type SettingEngine struct {
	loggerFactory logger.LoggerFactory
	logLevel      logger.LogLevel
}

//WithLoggerFactory 设置日志创建类
func WithLoggerFactory(v logger.LoggerFactory) SettingFunc {
	return func(setting *SettingEngine) {
		setting.loggerFactory = v
	}
}

//WithLogLevel 设置日志等级
func WithLogLevel(v logger.LogLevel) SettingFunc {
	return func(setting *SettingEngine) {
		setting.logLevel = v
	}
}
