package semaphorelogger

type emptyLoggerImpl struct {
}

func NewEmptyLogger() Logger {
	return &emptyLoggerImpl{}
}

func (l *emptyLoggerImpl) WithFields(fields map[string]interface{}) Logger {
	return l
}

func (l *emptyLoggerImpl) Log(level LogLevel, format string, args ...interface{}) {
}
