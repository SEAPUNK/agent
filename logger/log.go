package logger

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh/terminal"
)

const (
	nocolor   = "0"
	red       = "31"
	green     = "38;5;48"
	yellow    = "33"
	gray      = "38;5;251"
	graybold  = "1;38;5;251"
	lightgray = "38;5;243"
	cyan      = "1;36"
)

const (
	DateFormat = "2006-01-02 15:04:05"
)

var (
	mutex         = sync.Mutex{}
	windowsColors bool
)

type Logger interface {
	Debug(format string, v ...interface{})
	Error(format string, v ...interface{})
	Fatal(format string, v ...interface{})
	Notice(format string, v ...interface{})
	Warn(format string, v ...interface{})
	Info(format string, v ...interface{})

	WithFields(fields ...Field) Logger
	WithLevel(level Level) Logger
	Level() Level
}

type ConsoleLogger struct {
	level   Level
	exitFn  func(int)
	fields  Fields
	printer Printer
}

func NewConsoleLogger(printer Printer, exitFn func(int)) Logger {
	return &ConsoleLogger{
		level:   DEBUG,
		fields:  Fields{},
		printer: printer,
		exitFn:  exitFn,
	}
}

// WithFields returns a copy of the logger with the provided fields
func (l *ConsoleLogger) WithFields(fields ...Field) Logger {
	clone := *l
	clone.fields.Add(fields...)
	return &clone
}

// WithLevel returns a copy of the logger with the provided level
func (l *ConsoleLogger) WithLevel(level Level) Logger {
	clone := *l
	clone.level = level
	return &clone
}

func (l *ConsoleLogger) Debug(format string, v ...interface{}) {
	if l.level == DEBUG {
		l.printer.Print(DEBUG, fmt.Sprintf(format, v...), l.fields)
	}
}

func (l *ConsoleLogger) Error(format string, v ...interface{}) {
	l.printer.Print(ERROR, fmt.Sprintf(format, v...), l.fields)
}

func (l *ConsoleLogger) Fatal(format string, v ...interface{}) {
	l.printer.Print(FATAL, fmt.Sprintf(format, v...), l.fields)
	l.exitFn(1)
}

func (l *ConsoleLogger) Notice(format string, v ...interface{}) {
	if l.level <= NOTICE {
		l.printer.Print(NOTICE, fmt.Sprintf(format, v...), l.fields)
	}
}

func (l *ConsoleLogger) Info(format string, v ...interface{}) {
	if l.level <= INFO {
		l.printer.Print(INFO, fmt.Sprintf(format, v...), l.fields)
	}
}

func (l *ConsoleLogger) Warn(format string, v ...interface{}) {
	if l.level <= WARN {
		l.printer.Print(WARN, fmt.Sprintf(format, v...), l.fields)
	}
}

func (l *ConsoleLogger) Level() Level {
	return l.level
}

type Presenter interface {
	IsVisible(f Field) bool
	IsPrefix(f Field) bool
}

type DefaultPresenter struct{}

func (p *DefaultPresenter) IsVisible(f Field) bool { return true }
func (p *DefaultPresenter) IsPrefix(f Field) bool  { return true }

type Printer interface {
	Print(level Level, msg string, fields Fields)
}

type TextPrinter struct {
	Colors    bool
	Writer    io.Writer
	Presenter Presenter
}

func NewTextPrinter(w io.Writer) *TextPrinter {
	return &TextPrinter{
		Writer: w,
		Colors: ColorsSupported(),
	}
}

func (l *TextPrinter) Print(level Level, msg string, fields Fields) {
	now := time.Now().Format(DateFormat)

	var prefix string
	var line string
	var fieldStrs []string

	if l.Colors {
		levelColor := green
		messageColor := nocolor
		fieldColor := graybold

		switch level {
		case DEBUG:
			levelColor = gray
			messageColor = gray
		case NOTICE:
			levelColor = cyan
		case WARN:
			levelColor = yellow
		case ERROR:
			levelColor = red
		case FATAL:
			levelColor = red
			messageColor = red
		}

		if prefix != "" {
			line = fmt.Sprintf("\x1b[%sm%s %-6s\x1b[0m \x1b[%sm%s\x1b[0m \x1b[%sm%s\x1b[0m",
				levelColor, now, level, lightgray, prefix, messageColor, msg)
		} else {
			line = fmt.Sprintf("\x1b[%sm%s %-6s\x1b[0m \x1b[%sm%s\x1b[0m",
				fieldColor, now, level, messageColor, msg)
		}

		for _, field := range fields {
			fieldStrs = append(fieldStrs, fmt.Sprintf("\x1b[%sm%s=\x1b[0m\x1b[%sm%s\x1b[0m",
				lightgray, field.Key(), messageColor, field.String()))
		}
	} else {
		if prefix != "" {
			line = fmt.Sprintf("%s %-6s %s %s", now, level, prefix, msg)
		} else {
			line = fmt.Sprintf("%s %-6s %s", now, level, msg)
		}

		for _, field := range fields {
			if field.Key() == `agent_name` {
				continue
			}
			fieldStrs = append(fieldStrs, fmt.Sprintf("%s=%s", field.Key(), field.String()))
		}
	}

	// Make sure we're only outputting a line one at a time
	mutex.Lock()
	fmt.Fprint(l.Writer, line)
	if len(fields) > 0 {
		fmt.Fprintf(l.Writer, " %s", strings.Join(fieldStrs, " "))
	}
	fmt.Fprint(l.Writer, "\n")
	mutex.Unlock()
}

func ColorsSupported() bool {
	// Color support for windows is set in init
	if runtime.GOOS == "windows" && !windowsColors {
		return false
	}

	// Colors can only be shown if STDOUT is a terminal
	if terminal.IsTerminal(int(os.Stdout.Fd())) {
		return true
	}

	return false
}

type JSONPrinter struct {
	Writer    io.Writer
	Presenter Presenter
}

func NewJSONPrinter(w io.Writer) *JSONPrinter {
	return &JSONPrinter{
		Writer: w,
	}
}

func (p *JSONPrinter) Print(level Level, msg string, fields Fields) {
	var b strings.Builder

	b.WriteString(fmt.Sprintf(`"ts":%q,`, time.Now().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf(`"level":%q,`, level.String()))
	b.WriteString(fmt.Sprintf(`"msg":%q,`, msg))

	for _, field := range fields {
		b.WriteString(fmt.Sprintf(`%q:%q,`, field.Key(), field.String()))
	}

	// Make sure we're only outputting a line one at a time
	mutex.Lock()
	fmt.Fprintf(p.Writer, "{%s}\n", strings.TrimSuffix(b.String(), ","))
	mutex.Unlock()
}

var Discard = &ConsoleLogger{
	printer: &TextPrinter{
		Writer: ioutil.Discard,
	},
}
