package logger

type Logger struct{}

func New() *Logger {
	return &Logger{}
}

func Info(msg string) {}

func Error(msg string) {}
