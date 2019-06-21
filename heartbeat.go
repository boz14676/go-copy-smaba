package main

import (
    "github.com/gorilla/websocket"
    "strings"
    "time"
)

type Heartbeat struct {
    *UploadList
}

func Ticker(c *websocket.Conn) {
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            tUpload := make([]Upload, len(UploadSave.List))
            for _,upload := range UploadSave.List {
                if upload.IsOnWatch {
                    tUpload = append(tUpload, upload)
                }
            }

            if len(tUpload) > 0 {
                var resp RespWrap
                resp.setStatus(200)
                resp.respWrapper(strings.ToLower(ActUpload), UploadSave)
                if err := c.WriteJSON(resp); err != nil {
                    logger(wsLogTag).Error(err)
                }
            }
        }
    }
}