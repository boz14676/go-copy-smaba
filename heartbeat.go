package main

import (
    "strings"
    "time"
)

const HeartbeatSec = 1

func Ticker() {
    ticker := time.NewTicker(HeartbeatSec * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if len(UploadSave.List) == 0 {
                continue
            }

            var tUpload UploadList
            for _,upload := range UploadSave.List {
                if upload.IsOnWatch == true && upload.Status != StatusSucceeded && upload.IsPerCmplt != true {
                    // Stop keep watching if the transfer size equals the total size.
                    if upload.TransSize == upload.TotalSize {
                        upload.IsPerCmplt = true
                    }

                    // fmt.Printf("heartbeat-upload:%+v \n", upload)

                    tUpload.List = append(tUpload.List, upload)
                }
            }

            // fmt.Printf("heartbeat-t-upload:%+v", len(tUpload))

            if len(tUpload.List) > 0 {
                var resp RespWrap
                resp.setStatus(200, "The tasks is processing.")
                resp.respWrapper(strings.ToLower(ActWatch), tUpload)
                if err := WsConn.WriteJSON(resp); err != nil {
                    logger(wsLogTag).Error(err)
                }
            }
        }
    }
}