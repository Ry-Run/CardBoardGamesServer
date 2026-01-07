package logs

import (
	"common/config"
	"os"
	"time"

	"github.com/charmbracelet/log"
)

var logger *log.Logger

func InitLog(appName string) {
	// Stderr 控制台 Writer
	logger = log.New(os.Stderr)
	// 根据配置设置日志级别
	if config.Conf.Log.Level == "DEBUG" {
		logger.SetLevel(log.DebugLevel)
	} else {
		logger.SetLevel(log.InfoLevel)
	}
	// 设置日志前缀
	logger.SetPrefix(appName)
	// 打印时间
	log.SetReportTimestamp(true)
	// 设置打印时间格式
	log.SetTimeFormat(time.DateTime)
}

func Fatal(format string, args ...interface{}) {
	if len(args) == 0 {
		logger.Fatal(format)
	} else {
		logger.Fatalf(format, args)
	}
}

func Info(format string, args ...interface{}) {
	if len(args) == 0 {
		logger.Info(format)
	} else {
		logger.Infof(format, args)
	}
}

func Warn(format string, args ...interface{}) {
	if len(args) == 0 {
		logger.Warn(format)
	} else {
		logger.Warnf(format, args)
	}
}

func Debug(format string, args ...interface{}) {
	if len(args) == 0 {
		logger.Debug(format)
	} else {
		logger.Debugf(format, args)
	}
}

func Error(format string, args ...interface{}) {
	if len(args) == 0 {
		logger.Error(format)
	} else {
		logger.Errorf(format, args)
	}
}
