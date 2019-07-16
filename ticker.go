package main

import (
    // log "github.com/sirupsen/logrus"
    "strings"
    "time"
)

const (
    WatchTickerSec = 1
)

func Ticker() {
    watchTicker := time.NewTicker(WatchTickerSec * time.Second)

    defer func() {
        Wg.Done()
        watchTicker.Stop()
    }()

    for {
        select {

        // Watch processing of files transfer task.
        case <-watchTicker.C:
            if len(UploadSave.List) == 0 {
                continue
            }

            var tUpload UploadList
            for _, upload := range UploadSave.List {
                // Condition 1: The task status must be processing.
                // Condition 2: The task's sign of on-watch must be opened.
                // Condition 3: Stop keep watching if the transfer size equals the transfer size of previous.
                if upload.Status == StatusProcessing && upload.OnWatch == true && upload.TransSize != upload.TransSizePrev {
                    upload.TransSizePrev = upload.TransSize

                    tUpload.List = append(tUpload.List, upload)
                }
            }

            if len(tUpload.List) > 0 {
                var resp RespWrap
                resp.SetStatus(200, "The tasks is processing.")
                resp.RespWrapper(strings.ToLower(ActWatch), tUpload)
                resp.Send()
            }
        }
    }
}
