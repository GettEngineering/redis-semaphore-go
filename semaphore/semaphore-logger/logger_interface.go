package semaphorelogger

type LogLevel int

const (
	LogLevelError LogLevel = iota
	LogLevelInfo
	LogLevelDebug
)

//go:generate mockgen -source=./semaphore/semaphore-logger/logger_interface.go -destination=./semaphore/semaphore-logger/mock/logger_interface_mock.go Logger
type Logger interface {
	WithFields(fields map[string]interface{}) Logger
	Log(level LogLevel, format string, args ...interface{})
}
