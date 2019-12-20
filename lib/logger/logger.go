package logger

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/metrics"
)

var (
	loggerLevel  = flag.String("loggerLevel", "INFO", "Minimum level of errors to log. Possible values: INFO, ERROR, FATAL, PANIC")
	loggerFormat = flag.String("loggerFormat", "default", "Format for logs. Possible values: default, json")
)

// Init initializes the logger.
//
// Init must be called after flag.Parse()
//
// There is no need in calling Init from tests.
func Init() {
	validateLoggerLevel()
	validateLoggerFormat()
	go errorsLoggedCleaner()
	go logMessageWriter()
	logAllFlags()
}

func validateLoggerLevel() {
	switch *loggerLevel {
	case "INFO", "ERROR", "FATAL", "PANIC":
	default:
		// We cannot use logger.Panicf here, since the logger isn't initialized yet.
		panic(fmt.Errorf("FATAL: unsupported `-loggerLevel` value: %q; supported values are: INFO, ERROR, FATAL, PANIC", *loggerLevel))
	}
}

func validateLoggerFormat() {
	switch *loggerFormat {
	case "default", "json":
	default:
		// We cannot use logger.Pancif here, since the logger isn't initialized yet.
		panic(fmt.Errorf("FATAL: unsupported `-loggerFormat` value: %q; supported values are: default, json", *loggerFormat))
	}
}

var stdErrorLogger = log.New(&logWriter{}, "", 0)

// StdErrorLogger returns standard error logger.
func StdErrorLogger() *log.Logger {
	return stdErrorLogger
}

// Infof logs info message.
func Infof(format string, args ...interface{}) {
	logLevel("INFO", format, args...)
}

// Errorf logs error message.
func Errorf(format string, args ...interface{}) {
	logLevel("ERROR", format, args...)
}

// Fatalf logs fatal message and terminates the app.
func Fatalf(format string, args ...interface{}) {
	logLevel("FATAL", format, args...)
}

// Panicf logs panic message and panics.
func Panicf(format string, args ...interface{}) {
	logLevel("PANIC", format, args...)
}

func logLevel(level, format string, args ...interface{}) {
	if shouldSkipLog(level) {
		return
	}

	// rate limit ERROR log messages
	if level == "ERROR" {
		if n := atomic.AddUint64(&errorsLogged, 1); n > 10 {
			return
		}
	}

	msg := fmt.Sprintf(format, args...)
	logMessage(level, msg, 3)
}

func errorsLoggedCleaner() {
	for {
		time.Sleep(5 * time.Second)
		atomic.StoreUint64(&errorsLogged, 0)
	}
}

var errorsLogged uint64

type logWriter struct {
}

func (lw *logWriter) Write(p []byte) (int, error) {
	if !shouldSkipLog("ERROR") {
		logMessage("ERROR", string(p), 4)
	}
	return len(p), nil
}

func logMessage(level, msg string, skipframes int) {
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	levelLowercase := strings.ToLower(level)
	_, file, line, ok := runtime.Caller(skipframes)
	if !ok {
		file = "???"
		line = 0
	}
	path := filepath.FromSlash("/VictoriaMetrics/")
	if n := strings.Index(file, path); n >= 0 {
		// Strip /VictoriaMetrics/ prefix
		file = file[n+len(path):]
	}
	for len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	var logMsg string
	switch *loggerFormat {
	case "json":
		caller := fmt.Sprintf("%s:%d", file, line)
		logMsg = fmt.Sprintf(`{"ts":%q,"level":%q,"caller":%q,"msg":%q}`+"\n", timestamp, levelLowercase, caller, msg)
	default:
		logMsg = fmt.Sprintf("%s\t%s\t%s:%d\t%s\n", timestamp, levelLowercase, file, line, msg)
	}

	select {
	case logMessageCh <- logMsg:
	default:
		// Writing to log can stuck if the log output isn't consumed in timely manner.
		// Handle this case via `vm_log_messages_dropped_total`
		logMessagesDropped.Inc()
	}

	// Increment vm_log_messages_total
	location := fmt.Sprintf("%s:%d", file, line)
	counterName := fmt.Sprintf(`vm_log_messages_total{app_version=%q, level=%q, location=%q}`, buildinfo.Version, levelLowercase, location)
	metrics.GetOrCreateCounter(counterName).Inc()

	switch level {
	case "PANIC":
		panic(errors.New(msg))
	case "FATAL":
		os.Exit(-1)
	}
}

var logMessagesDropped = metrics.NewCounter(`vm_log_messages_dropped_total`)

func logMessageWriter() {
	for msg := range logMessageCh {
		fmt.Fprint(os.Stderr, msg)
	}
}

var logMessageCh = make(chan string, 100)

func shouldSkipLog(level string) bool {
	switch *loggerLevel {
	case "ERROR":
		switch level {
		case "ERROR", "FATAL", "PANIC":
			return false
		default:
			return true
		}
	case "FATAL":
		switch level {
		case "FATAL", "PANIC":
			return false
		default:
			return true
		}
	case "PANIC":
		return level != "PANIC"
	default:
		return false
	}
}
