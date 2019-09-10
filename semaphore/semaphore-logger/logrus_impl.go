package semaphorelogger

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

type logrusImpl struct {
	logger     *logrus.Entry
	bindingKey string
}

func NewLogrusLogger(logger *logrus.Logger, level LogLevel, bindingKey string) Logger {
	if logger == nil {
		logger = logrus.StandardLogger()
	}

	logger.SetLevel(parseLevel(level))

	return &logrusImpl{logger: logger.WithField("binding_key", bindingKey), bindingKey: bindingKey}
}

func (l *logrusImpl) WithFields(fields map[string]interface{}) Logger {
	return &logrusImpl{logger: l.logger.WithFields(logrus.Fields(fields)).WithField("binding_key", l.bindingKey), bindingKey: l.bindingKey}
}

func (l *logrusImpl) Log(level LogLevel, format string, args ...interface{}) {
	l.logger.Log(parseLevel(level), fmt.Sprintf(format, args...))
}

func parseLevel(level LogLevel) logrus.Level {
	switch level {

	case LogLevelError:
		return logrus.ErrorLevel
	case LogLevelInfo:
		return logrus.InfoLevel
	case LogLevelDebug:
		return logrus.DebugLevel
	default:
		return logrus.TraceLevel
	}
}
