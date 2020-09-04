package logger

import "fmt"

//DefaultFactory ...
type DefaultFactory struct {
}

type defaultLogger struct {
}

//NewDefaultFactory ...
func NewDefaultFactory() *DefaultFactory {
	return &DefaultFactory{}
}

//NewLogger ...
func (f *DefaultFactory) NewLogger(scope string) Logger {
	return defaultLogger{}
}

func (log defaultLogger) Trace(msg string) {
	fmt.Println(msg)
}

func (log defaultLogger) Tracef(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func (log defaultLogger) Debug(msg string) {
	fmt.Println(msg)
}

func (log defaultLogger) Debugf(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func (log defaultLogger) Info(msg string) {
	fmt.Println(msg)
}

func (log defaultLogger) Infof(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func (log defaultLogger) Warn(msg string) {
	fmt.Println(msg)
}

func (log defaultLogger) Warnf(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func (log defaultLogger) Error(msg string) {
	fmt.Println(msg)
}

func (log defaultLogger) Errorf(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}
