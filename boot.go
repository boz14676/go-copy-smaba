package main

import (
    "fmt"
    log "github.com/sirupsen/logrus"
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
    // Log as JSON instead of the default ASCII formatter.
    // log.SetFormatter(&log.JSONFormatter{})

    // Output to stdout instead of the default stderr.
    // Can be any io.Writer, see below for File example.
    log.SetOutput(os.Stdout)

    // Only log the debug(-warning) severity or above.
    log.SetLevel(log.DebugLevel)

    log.SetReportCaller(true)
    log.SetFormatter(&log.TextFormatter{
        CallerPrettyfier: func(f *runtime.Frame) (string, string) {
            filename := f.File
            n := 0
            for i := len(filename) - 1; i > 0; i-- {
                if filename[i] == '/' {
                    n++
                    if n >= 2 {
                        filename = filename[i+1:]
                        break
                    }
                }
            }
            return fmt.Sprintf("%s()", f.Function), fmt.Sprintf("%s:%d", filename, f.Line)
        },
    })
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
    defer DBInstance.db.Close()
}
