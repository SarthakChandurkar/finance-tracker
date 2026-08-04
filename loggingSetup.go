package main

import (
	"io"
	"log"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

// setupLogging routes every log.Print*/log.Fatal* call to BOTH stdout and
// a rotating log file. Call this once, at the very top of main(), before
// anything else runs.
func setupLogging() {
	fileLogger := &lumberjack.Logger{
		Filename:   "logs/app.log",
		MaxSize:    5,    // megabytes — once app.log reaches this size, it's rotated out
		MaxBackups: 3,    // how many old rotated files to keep around
		MaxAge:     14,   // days — rotated files older than this get deleted too
		Compress:   true, // gzip old rotated files, so backups take less disk space
	}

	log.SetOutput(io.MultiWriter(os.Stdout, fileLogger))
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
