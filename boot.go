package main

import (
    log "github.com/sirupsen/logrus"
    logUnderlying "log"
    "os"
    "runtime"
    "sync"
)

var (
    Wg sync.WaitGroup
    MaxWorker = runtime.NumCPU() * 2
)

func logger(optional ...string) *log.Entry {
    var fields log.Fields

    if len(optional) > 0 {
        logTag := optional[0]
        if logTag != "" {
            fields = log.Fields{"subject": logTag}
        }
    }

    return log.WithFields(fields)
}

func init() {
    logUnderlying.SetFlags(logUnderlying.LstdFlags | logUnderlying.Lshortfile)
    // Log as JSON instead of the default ASCII formatter.
    // log.SetFormatter(&log.JSONFormatter{})

    // Output to stdout instead of the default stderr.
    // Can be any io.Writer, see below for File example.
    log.SetOutput(os.Stdout)

    // Only log the debug(-warning) severity or above.
    log.SetLevel(log.DebugLevel)
}

func main() {
    // Launch websocket.
    Wg.Add(1)
    go openWs()

    // Monitor tasks of files transfer.
    Wg.Add(MaxWorker)
    for i := 1; i <= MaxWorker; i++ {
        go Monitor()
    }

    // Ticker
    Wg.Add(1)
    go Ticker()

    Wg.Wait()

    // Close database instance.
    defer func() {
        _ = DBInstance.db.Close()
    }()
}
