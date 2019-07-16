package main

import (
    "encoding/json"
    "errors"
    "flag"
    "github.com/gorilla/websocket"
    log "github.com/sirupsen/logrus"
    "io/ioutil"
    "net/http"
    "strings"
    "time"
)

const (
    // Websocket log tag.
    wsLogTag = "ws-server"

    // Action for upload.
    ActUpload = "UPLOAD"

    // Action for resume.
    ActResume = "RESUME"

    // Action for watch of heartbeat-pack.
    ActWatch = "WATCH"

    // Action for offwatch of heartbeat-pack.
    ActOffWatch = "OFFWATCH"

    // Action for list of uploaded.
    ActList = "LIST"

    // Action for pause for transfer files.
    ActPause = "PAUSE"

    // Action for abort for transfer files.
    ActAbort = "REMOVE"
)

var (
    addr = flag.String("addr", "localhost:8080", "http service address")

    upgrader = websocket.Upgrader{
        CheckOrigin: func(r *http.Request) bool {
            return true
        },
    }

    // Upload message global variable.
    UploadSave UploadList

    // Connection global variable for websocket.
    WsConn *websocket.Conn
)

// Client message.
type Message struct {
    Method string `json:"cmd"`
}

// Client message for upload.
type UploadMsg struct {
    Opt UploadList `json:"opt"`
}

// Client message for list of uploaded.
type ListMsg struct {
    Opt ListWrap `json:"opt"`
}

// Client message for request params of list.
type ListWrap struct {
    Offset    int64  `json:"offset"`
    Limit     int64  `json:"amt"`
    Sort      string `json:"sort"`
    Created   int64  `json:"created"`
    StartedAt int64  `json:"from"`
    EndedAt   int64  `json:"to"`
    Status    int8   `json:"status"`
}

type ListRespMsg struct {
    List interface{} `json:"list"`
}

// Initialized message of list.
func (listMsg *ListWrap) Init() {
    listMsg.Offset = -1
    listMsg.Limit = -1
    listMsg.Status = -1
}

// func (message *Message) log(act string, code int32) {
//     log.WithFields(log.Fields{
//         "subject": wsLogTag,
//         "act": ActUpload,
//     }).Debug("act-upload is calling: ", string(recv))
// }

// Websocket service.
func ws(w http.ResponseWriter, r *http.Request) {
    c, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        logger(wsLogTag).Error("upgrade:", err)
        return
    }

    WsConn = c

    defer c.Close()

    for {
        var m Message
        var resp RespWrap
        var recv []byte

        // Get io.Reader of client message.
        _, r, err := c.NextReader()

        if err != nil {
            logger(wsLogTag).Error(err)
            _ = c.Close()
            break
        }

        // Get method which is from io.Reader of client message.
        recv, err = ioutil.ReadAll(r)
        if err = json.Unmarshal(recv, &m); err != nil {
            resp.SetStatus(400, "The request data is illegal: " + string(recv))
            resp.Send()
        }

        // Main process.
        switch strings.ToUpper(m.Method) {

        // Process files upload.
        case ActUpload:
            var uploadMsg UploadMsg

            // Log for request "upload" message.
            logger(wsLogTag).WithFields(log.Fields{"act": ActUpload}).Debug("act-upload is calling: ", string(recv))

            err = json.Unmarshal(recv, &uploadMsg)
            if err != nil || uploadMsg.Opt.List == nil || len(uploadMsg.Opt.List) == 0 {
                logger(wsLogTag).WithFields(log.Fields{"act": ActUpload}).Error(errors.New("data illegal in upload message: " + string(recv)))

                resp.SetStatus(400, "The request data is illegal in upload message: " + string(recv))
                resp.RespWrapper(strings.ToLower(ActUpload))
                resp.Send()
            } else {
                UploadSave.append(uploadMsg.Opt.List)
                UploadSave.Process()
            }

        // Resuming files transferred.
        case ActResume:
            var uploadMsg UploadMsg

            // Log for request "resume" message.
            logger(wsLogTag).WithFields(log.Fields{"act": ActResume}).Debug("act-resume is calling: ", string(recv))

            err = json.Unmarshal(recv, &uploadMsg)
            if err != nil || uploadMsg.Opt.List == nil || len(uploadMsg.Opt.List) == 0 {
                logger(wsLogTag).WithFields(log.Fields{"act": ActResume}).Error(errors.New("data illegal in resume message: " + string(recv)))

                resp.SetStatus(400, "The request data is illegal in resume message: " + string(recv))
                resp.RespWrapper(strings.ToLower(ActResume))
                resp.Send()
            } else {
                // Dictionary table for Upload global save.
                set := make(map[int64]*Upload)
                for _, upload := range UploadSave.List {
                    set[upload.UUID] = upload
                }

                for _, upload := range uploadMsg.Opt.List {
                    // If upload already in Upload global save.
                    if set[upload.UUID] != nil {
                        _ = set[upload.UUID].EmitResuming()
                    } else {
                        if err := upload.EmitResuming(); err == nil {
                            UploadSave.List = append(UploadSave.List, upload)
                        }
                    }
                }

                UploadSave.Process()
            }

        // Get list for file tasks.
        case ActList:
            var listMsg ListMsg
            listMsg.Opt.Init()

            logger(wsLogTag).WithFields(log.Fields{"act": ActList}).Debug("act-list is calling: ", string(recv))

            err = json.Unmarshal(recv, &listMsg)
            if err != nil {
                logger(wsLogTag).Error(err)

                resp.SetStatus(400, "The request data is illegal in list message: " + string(recv))
                resp.RespWrapper(strings.ToLower(ActList))
                resp.Send()

            } else {
                var upload Upload
                uploadList, err := upload.List(listMsg.Opt)
                if err != nil {
                    logger(wsLogTag).WithFields(log.Fields{"act": ActList}).Error(err)

                    resp.SetStatus(500, "Response errors occurred")
                    resp.RespWrapper(strings.ToLower(ActList))
                    resp.Send()
                } else {
                    var listResp ListRespMsg
                    listResp.List = uploadList

                    resp.SetStatus(200)
                    resp.RespWrapper(strings.ToLower(ActList), listResp)
                    resp.Send()
                }
            }

        // Watching for processing of files transfer.
        case ActWatch:
            var watchMsg UploadMsg
            err = json.Unmarshal(recv, &watchMsg)

            logger(wsLogTag).WithFields(log.Fields{"act": ActWatch}).Debug("act-watch is calling: ", string(recv))

            if err != nil {
                logger(wsLogTag).WithFields(log.Fields{"act": ActWatch}).Error(errors.New("data illegal in act-watch message: " + string(recv)))

                resp.SetStatus(400, "The Request data is illegal in watch message: " + string(recv))
                resp.RespWrapper(strings.ToLower(ActWatch))
                resp.Send()
            } else {
                // Check global variable "uploadSave" if empty.
                if UploadSave.List == nil || len(UploadSave.List) == 0 {
                    logger(wsLogTag).WithFields(log.Fields{"act": ActWatch}).Error(errors.New("no job are processing in message of act-watch"))

                    resp.SetStatus(403, "no job are processing")
                    resp.RespWrapper(strings.ToLower(ActWatch))
                    resp.Send()
                } else {
                    // Watching all the processing tasks when opt-msg is empty.
                    if watchMsg.Opt.List == nil {
                        for _, upload := range UploadSave.List {
                            upload.EmitWatch()
                        }
                    } else {
                        // Watching specified upload from uuid.
                        set := make(map[int64]*Upload)
                        for _, upload := range UploadSave.List {
                            set[upload.UUID] = upload
                        }

                        for _, _upload := range watchMsg.Opt.List {
                            if set[_upload.UUID] != nil {
                                set[_upload.UUID].EmitWatch()
                            }
                        }
                    }
                }

            }

        // Stop watching for processing of files transfer.
        case ActOffWatch:
            var watchMsg UploadMsg
            err = json.Unmarshal(recv, &watchMsg)

            logger(wsLogTag).WithFields(log.Fields{"act": ActOffWatch}).Debug("act-off-watch is calling: ", string(recv))

            if err != nil {
                logger(wsLogTag).WithFields(log.Fields{"act": ActOffWatch}).Error(errors.New("data illegal in act-off-watch message: " + string(recv)))

                resp.SetStatus(400, "The Request data is illegal in off-watch message: " + string(recv))
                resp.RespWrapper(strings.ToLower(ActOffWatch))
                resp.Send()
            } else {
                if UploadSave.List == nil || len(UploadSave.List) == 0 {
                    logger(wsLogTag).WithFields(log.Fields{"act": ActOffWatch}).Error(errors.New("no job are processing in act-off-watch message"))

                    resp.SetStatus(403, "no job are processing")
                    resp.RespWrapper(strings.ToLower(ActWatch))
                    resp.Send()
                } else {
                    // Stop Watching all the processing tasks when opt-msg is empty.
                    if watchMsg.Opt.List == nil {
                        for _, upload := range UploadSave.List {
                            upload.EmitOffWatch()
                        }
                    } else {
                        // Stop Watching specified upload from uuid.
                        set := make(map[int64]*Upload)
                        for _, upload := range UploadSave.List {
                            set[upload.UUID] = upload
                        }

                        for _, _upload := range watchMsg.Opt.List {
                            if set[_upload.UUID] != nil {
                                set[_upload.UUID].EmitOffWatch()
                            }
                        }
                    }
                }
            }

        // Pausing files transferred.
        case ActPause:
            var uploadMsg UploadMsg
            err = json.Unmarshal(recv, &uploadMsg)

            logger(wsLogTag).WithFields(log.Fields{"act": ActPause}).Debug("act-pause is calling: ", string(recv))

            if err != nil {
                logger(wsLogTag).WithFields(log.Fields{"act": ActPause}).Error(errors.New("data illegal in act-pause message: " + string(recv)))

                resp.SetStatus(400, "The request data is illegal in message of act-pause: " + string(recv))
                resp.RespWrapper(strings.ToLower(ActPause))
                resp.Send()
            } else {
                if UploadSave.List == nil || len(UploadSave.List) == 0 {
                    logger(wsLogTag).WithFields(log.Fields{"act": ActPause}).Error(errors.New("no job are processing in act-pause message"))

                    resp.SetStatus(403, "no job are processing")
                    resp.RespWrapper(strings.ToLower(ActPause))
                    resp.Send()
                } else {
                    // Pausing all the processing tasks when opt-msg is empty.
                    if uploadMsg.Opt.List == nil {
                        for _, upload := range UploadSave.List {
                            upload.EmitPause()
                        }
                    } else {
                        // Pausing specified processing task from uuid.
                        set := make(map[int64]*Upload)
                        for _, upload := range UploadSave.List {
                            set[upload.UUID] = upload
                        }

                        for _, _upload := range uploadMsg.Opt.List {
                            if set[_upload.UUID] != nil {
                                set[_upload.UUID].EmitPause()
                            }
                        }
                    }
                }
            }

        // Aborting files transferred.
        case ActAbort:
            var uploadMsg UploadMsg
            err = json.Unmarshal(recv, &uploadMsg)

            logger(wsLogTag).WithFields(log.Fields{"act": ActAbort}).Debug("act-abort is calling: ", string(recv))

            if err != nil {
                logger(wsLogTag).WithFields(log.Fields{"act": ActAbort}).Error(errors.New("data illegal in act-abort message: " + string(recv)))

                resp.SetStatus(400, "The request data is illegal in message of act-abort: " + string(recv))
                resp.RespWrapper(strings.ToLower(ActPause))
                resp.Send()
            } else {
                // Aborting specified processing task from uuid.
                set := make(map[int64]*Upload)
                for _, upload := range UploadSave.List {
                    set[upload.UUID] = upload
                }

                for _, _upload := range uploadMsg.Opt.List {
                    if set[_upload.UUID] != nil {
                        set[_upload.UUID].EmitAbort()
                    } else {
                        if err = _upload.Find(); err != nil {
                            _upload.SendMsg(ActAbort, 500, 3008)
                        } else {
                            if err = _upload.Setup(); err != nil {
                                _upload.SendMsg(ActAbort, 500, 2003)
                            } else {
                                _upload.erase()
                            }
                        }
                    }
                }
            }

        default:
            resp.SetStatus(400, "request data illegal: " + string(recv))
            resp.Send()
        }

    }
}

// Response wrap struct.
type RespWrap struct {
    Cmd       string      `json:"cmd,omitempty"`
    Timestamp int64       `json:"timestamp"`
    Code      int16       `json:"code"`
    Message   string      `json:"msg"`
    Data      interface{} `json:"data,omitempty"`
}

// Response status setting.
func (resp *RespWrap) SetStatus(code int16, optional ...string) {
    resp.Code = code

    if len(optional) == 1 && optional[0] != "" {
        resp.Message = optional[0]
    } else {
        if resp.Code == 200 {
            resp.Message = "the request has succeeded."
        } else {
            resp.Message = "the request has failed."
        }
    }
}

// Response wrapper.
func (resp *RespWrap) RespWrapper(args ...interface{}) {
    for i, arg := range args {
        // Cmd of resp assignment.
        if i == 0 {
            switch t := arg.(type) {
            case string:
                resp.Cmd = t
            }
        }

        // Data of resp assignment.
        if i == 1 {
            switch t := arg.(type) {
            case interface{}:
                resp.Data = t
            }
        }
    }

    resp.Timestamp = time.Now().Unix()
}

// Send
func (resp *RespWrap) Send() {
    if err := WsConn.WriteJSON(resp); err != nil {
        logger(wsLogTag).Error(err)
    }
}

func openWs() {
    defer Wg.Done()

    flag.Parse()
    http.HandleFunc("/files_trans", ws)
    logger(wsLogTag).Fatal(http.ListenAndServe(*addr, nil))
}

