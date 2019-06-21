package main

import (
    log "github.com/sirupsen/logrus"
    "os"
    "runtime"
    "sync"
)

var Wg sync.WaitGroup
var MaxWorker = runtime.NumCPU()

// func checkErr(logTag string, err error) {
//     if err != nil {
//         // log.WithFields({"subject": }).Panic(err)
//     }
// }

func checkErr(LogTag string, err error) {
    if err != nil {
        logger(LogTag).Fatal(err)
    }
}

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

    // Output to stdout instead of the default stderr
    // Can be any io.Writer, see below for File example
    log.SetOutput(os.Stdout)

    // Only log the debug(-warning) severity or above.
    log.SetLevel(log.DebugLevel)
}

func main() {

    Wg.Add(1)
    go openWs()

    Wg.Add(MaxWorker)
    for i := 1; i <= MaxWorker; i++ {
        go Monitor()
    }

    Wg.Wait()
}

