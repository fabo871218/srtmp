package logger

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

//DefaultFactory ...
type DefaultFactory struct {
}

type defaultLogger struct {
	level LogLevel
}

//NewDefaultFactory ...
func NewDefaultFactory() *DefaultFactory {
	return &DefaultFactory{}
}

//NewLogger ...
func (f *DefaultFactory) NewLogger(scope LogLevel) Logger {
	return defaultLogger{
		level: scope,
	}
}

func (log defaultLogger) Trace(msg string) {
	log.output(2, LogLevelTrace, msg)
}

func (log defaultLogger) Tracef(format string, args ...interface{}) {
	log.output(2, LogLevelTrace, fmt.Sprintf(format, args...))
}

func (log defaultLogger) Debug(msg string) {
	log.output(2, LogLevelDebug, msg)
}

func (log defaultLogger) Debugf(format string, args ...interface{}) {
	log.output(2, LogLevelDebug, fmt.Sprintf(format, args...))
}

func (log defaultLogger) Info(msg string) {
	log.output(2, LogLevelInfo, msg)
}

func (log defaultLogger) Infof(format string, args ...interface{}) {
	log.output(2, LogLevelInfo, fmt.Sprintf(format, args...))
}

func (log defaultLogger) Warn(msg string) {
	log.output(2, LogLevelWarn, msg)
}

func (log defaultLogger) Warnf(format string, args ...interface{}) {
	log.output(2, LogLevelWarn, fmt.Sprintf(format, args...))
}

func (log defaultLogger) Error(msg string) {
	log.output(2, LogLevelError, msg)
}

func (log defaultLogger) Errorf(format string, args ...interface{}) {
	log.output(2, LogLevelError, fmt.Sprintf(format, args...))
}

func (log defaultLogger) output(callDepth int, level LogLevel, s string) {
	if log.level < level {
		return
	}
	var (
		file string
		line int
		ok   bool
	)

	_, file, line, ok = runtime.Caller(callDepth)
	if !ok {
		file = "???"
		line = 0
	}

	index := strings.LastIndex(file, "/")
	fmt.Printf("%s %s %s:%d     %s\n", time.Now().Format("2006-01-02 15:04:05.000"), level.String(), file[index+1:], line, s)
}
