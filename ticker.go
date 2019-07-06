package main

import (
    // log "github.com/sirupsen/logrus"
    "strings"
    "time"
)

const (
    WatchTickerSec = 1
    cleanTickerSec = 60
)

func Ticker() {
    watchTicker := time.NewTicker(WatchTickerSec * time.Second)
    cleanTicker := time.NewTicker(cleanTickerSec * time.Second)

    defer func() {
        Wg.Done()
        watchTicker.Stop()
        cleanTicker.Stop()
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
                if upload.IsOnWatch == true && upload.Status != StatusSucceeded && upload.IsPerCmplt != true {
                    // Stop keep watching if the transfer size equals the total size.
                    if upload.TransSize == upload.TotalSize {
                        upload.IsPerCmplt = true
                    }

                    tUpload.List = append(tUpload.List, upload)
                }
            }

            if len(tUpload.List) > 0 {
                var resp RespWrap
                resp.setStatus(200, "The tasks is processing.")
                resp.respWrapper(strings.ToLower(ActWatch), tUpload)
                if err := WsConn.WriteJSON(resp); err != nil {
                    logger(wsLogTag).Error(err)
                }
            }

        case <-cleanTicker.C:
            if ProcCounter == 0 {
                UploadSave.Lock()
                UploadSave.List = []*Upload{}
                UploadSave.Unlock()
            }

            // logger().WithFields(log.Fields{"UploadSave.List": UploadSave.List,}).Debug("CleanTicker-BP")
        }
    }
}
