package main

import (
    "encoding/json"
    "errors"
    "flag"
    "github.com/gorilla/websocket"
    log "github.com/sirupsen/logrus"
    "html/template"
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
type UploadOptMsg struct {
    Opt UploadList `json:"opt"`
}

// Client message for list of uploaded.
type ListOptMsg struct {
    Opt ListMsg `json:"opt"`
}

// Client message for request params of list.
type ListMsg struct {
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
func (listMsg *ListMsg) Init() {
    listMsg.Offset = -1
    listMsg.Limit = -1
    listMsg.Status = -1
}

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
            resp.setStatus(400, "The request data is illegal: " + string(recv))
            if err := c.WriteJSON(resp); err != nil {
                logger(wsLogTag).Error(err)
            }
        }

        // Main process.
        switch strings.ToUpper(m.Method) {

        // Process files upload.
        case ActUpload:
            var uploadMsg UploadOptMsg

            // Log for request "upload" message.
            logger(wsLogTag).WithFields(log.Fields{"act": ActUpload}).Debug("act-upload is calling: ", string(recv))

            err = json.Unmarshal(recv, &uploadMsg)
            if err != nil || uploadMsg.Opt.List == nil || len(uploadMsg.Opt.List) == 0 {
                logger(wsLogTag).WithFields(log.Fields{"act": ActUpload}).Error(errors.New("data illegal in upload message: " + string(recv)))

                resp.setStatus(400, "The request data is illegal in upload message: " + string(recv))
                resp.respWrapper(strings.ToLower(ActUpload))
                if err := c.WriteJSON(resp); err != nil {
                    logger(wsLogTag).Error(err)
                }
            } else {
                UploadSave.Lock()

                ProcCounter += len(uploadMsg.Opt.List)

                UploadSave.List = append(UploadSave.List, uploadMsg.Opt.List...)

                hasErr := UploadSave.Process()

                UploadSave.Unlock()

                if !hasErr {
                    resp.setStatus(200)
                    resp.respWrapper(strings.ToLower(ActUpload), UploadSave)
                    if err := c.WriteJSON(resp); err != nil {
                        logger(wsLogTag).Error(err)
                    }
                }
            }

        // Resuming files transferred.
        case ActResume:
            var uploadMsg UploadOptMsg

            // Log for request "resume" message.
            logger(wsLogTag).WithFields(log.Fields{"act": ActResume}).Debug("act-resume is calling: ", string(recv))

            err = json.Unmarshal(recv, &uploadMsg)
            if err != nil || uploadMsg.Opt.List == nil || len(uploadMsg.Opt.List) == 0 {
                logger(wsLogTag).WithFields(log.Fields{"act": ActResume}).Error(errors.New("data illegal in resume message: " + string(recv)))

                resp.setStatus(400, "The request data is illegal in resume message: " + string(recv))
                resp.respWrapper(strings.ToLower(ActResume))
                if err := c.WriteJSON(resp); err != nil {
                    logger(wsLogTag).Error(err)
                }
            } else {
                UploadSave.Lock()

                // Dictionary table for Upload global save.
                set := make(map[int64]*Upload)
                for _, upload := range UploadSave.List {
                    set[upload.UUID] = upload
                }

                for _, upload := range uploadMsg.Opt.List {
                    // If upload already in Upload global save.
                    if set[upload.UUID] != nil {
                        if err = set[upload.UUID].EmitResuming(); err != nil {
                            upload.log(wsLogTag).WithFields(log.Fields{"act": ActResume}).Error(err.Error())

                            // Abandoned for upload task if there is error occurred.
                            set[upload.UUID].Abandon()

                            // Send message to websocket client.
                            var resp RespWrap
                            resp.setStatus(500, err.Error())

                            var ulWrap UploadList
                            ulWrap.Fill(upload)

                            resp.respWrapper(strings.ToLower(ActResume), ulWrap)
                            if err := WsConn.WriteJSON(resp); err != nil {
                                logger(wsLogTag).Error(err)
                            }
                        }

                        logger("testing").Debug("Find exists upload: ", set[upload.UUID])

                    } else {
                        if err = upload.EmitResuming(); err != nil {
                            upload.log(wsLogTag).WithFields(log.Fields{"act": ActResume}).Error(err.Error())

                            // Send message to websocket client.
                            var resp RespWrap
                            resp.setStatus(500, err.Error())

                            var ulWrap UploadList
                            ulWrap.Fill(upload)

                            resp.respWrapper(strings.ToLower(ActResume), ulWrap)
                            if err := WsConn.WriteJSON(resp); err != nil {
                                logger(wsLogTag).Error(err)
                            }
                        } else {
                            ProcCounter++
                            UploadSave.List = append(UploadSave.List, upload)
                        }

                        logger("testing").Debug("Push a new upload: ", upload)

                    }
                }

                UploadSave.Process()

                UploadSave.Unlock()
            }

        // Get list for file tasks.
        case ActList:
            var listMsg ListOptMsg
            listMsg.Opt.Init()

            logger(wsLogTag).WithFields(log.Fields{"act": ActList}).Debug("act-list is calling: ", string(recv))

            err = json.Unmarshal(recv, &listMsg)
            if err != nil {
                logger(wsLogTag).Error(err)

                resp.setStatus(400, "The request data is illegal in list message: " + string(recv))
                resp.respWrapper(strings.ToLower(ActList))
                if err := c.WriteJSON(resp); err != nil {
                    logger(wsLogTag).Error(err)
                }

            } else {
                var upload Upload
                uploadList, err := upload.List(listMsg.Opt)
                if err != nil {
                    logger(wsLogTag).WithFields(log.Fields{"act": ActList}).Error(err)

                    resp.setStatus(500, "Response errors occurred: " + string(recv))
                    resp.respWrapper(strings.ToLower(ActList))
                    if err := c.WriteJSON(resp); err != nil {
                        logger(wsLogTag).Error(err)
                    }
                } else {
                    var listResp ListRespMsg
                    listResp.List = uploadList

                    resp.setStatus(200)
                    resp.respWrapper(strings.ToLower(ActList), listResp)

                    if err := c.WriteJSON(resp); err != nil {
                        logger(wsLogTag).Error(err)
                    }
                }
            }

        // Watching for processing of files transfer.
        case ActWatch:
            var watchMsg UploadOptMsg
            err = json.Unmarshal(recv, &watchMsg)

            logger(wsLogTag).WithFields(log.Fields{"act": ActWatch}).Debug("act-watch is calling: ", string(recv))

            if err != nil {
                logger(wsLogTag).WithFields(log.Fields{"act": ActWatch}).Error(errors.New("data illegal in act-watch message: " + string(recv)))

                resp.setStatus(400, "The Request data is illegal in watch message: " + string(recv))
                resp.respWrapper(strings.ToLower(ActWatch))
                if err := c.WriteJSON(resp); err != nil {
                    logger(wsLogTag).Error(err)
                }
            } else {
                // Check global variable "uploadSave" if empty.
                if UploadSave.List == nil || len(UploadSave.List) == 0 {
                    logger(wsLogTag).WithFields(log.Fields{"act": ActWatch}).Error(errors.New("no job are processing in message of act-watch"))

                    resp.setStatus(403, "no job are processing")
                    resp.respWrapper(strings.ToLower(ActWatch))

                    if err := c.WriteJSON(resp); err != nil {
                        logger(wsLogTag).Error(err)
                    }
                } else {
                    UploadSave.Lock()

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

                    UploadSave.Unlock()
                }

            }

        // Stop watching for processing of files transfer.
        case ActOffWatch:
            var watchMsg UploadOptMsg
            err = json.Unmarshal(recv, &watchMsg)

            logger(wsLogTag).WithFields(log.Fields{"act": ActOffWatch}).Debug("act-off-watch is calling: ", string(recv))

            if err != nil {
                logger(wsLogTag).WithFields(log.Fields{"act": ActOffWatch}).Error(errors.New("data illegal in act-off-watch message: " + string(recv)))

                resp.setStatus(400, "The Request data is illegal in off-watch message: " + string(recv))
                resp.respWrapper(strings.ToLower(ActOffWatch))
                if err := c.WriteJSON(resp); err != nil {
                    logger(wsLogTag).Error(err)
                }
            } else {
                if UploadSave.List == nil || len(UploadSave.List) == 0 {
                    logger(wsLogTag).WithFields(log.Fields{"act": ActOffWatch}).Error(errors.New("no job are processing in act-off-watch message"))

                    resp.setStatus(403, "no job are processing")
                    resp.respWrapper(strings.ToLower(ActWatch))

                    if err := c.WriteJSON(resp); err != nil {
                        logger(wsLogTag).Error(err)
                    }
                } else {
                    UploadSave.Lock()

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

                    UploadSave.Unlock()
                }
            }

        // Pausing files transferred.
        case ActPause:
            var uploadMsg UploadOptMsg
            err = json.Unmarshal(recv, &uploadMsg)

            logger(wsLogTag).WithFields(log.Fields{"act": ActPause}).Debug("act-pause is calling: ", string(recv))

            if err != nil {
                logger(wsLogTag).WithFields(log.Fields{"act": ActPause}).Error(errors.New("data illegal in act-pause message: " + string(recv)))

                resp.setStatus(400, "The request data is illegal in message of act-pause: " + string(recv))
                resp.respWrapper(strings.ToLower(ActPause))
                if err := c.WriteJSON(resp); err != nil {
                    logger(wsLogTag).Error(err)
                }
            } else {
                if UploadSave.List == nil || len(UploadSave.List) == 0 {
                    logger(wsLogTag).WithFields(log.Fields{"act": ActPause}).Error(errors.New("no job are processing in act-pause message"))

                    resp.setStatus(403, "There is no job are processing.")
                    resp.respWrapper(strings.ToLower(ActPause))

                    if err := c.WriteJSON(resp); err != nil {
                        logger(wsLogTag).Error(err)
                    }
                } else {
                    UploadSave.Lock()

                    // Pausing all the processing tasks when opt-msg is empty.
                    if uploadMsg.Opt.List == nil {
                        for _, upload := range UploadSave.List {
                            if !upload.EmitPause() {
                                upload.log(wsLogTag).WithFields(log.Fields{"act": ActPause}).Error(errors.New("pausing failed in pause message"))

                                var resp RespWrap
                                resp.setStatus(500, "Pausing failed in pause message.")

                                var ulWrap UploadList
                                ulWrap.Fill(upload)

                                resp.respWrapper(strings.ToLower(ActPause), ulWrap)
                                if err := WsConn.WriteJSON(resp); err != nil {
                                    logger(wsLogTag).Error(err)
                                }
                            }
                        }
                    } else {
                        // Pausing specified processing task from uuid.
                        set := make(map[int64]*Upload)
                        for _, upload := range UploadSave.List {
                            set[upload.UUID] = upload
                        }

                        for _, _upload := range uploadMsg.Opt.List {
                            if set[_upload.UUID] != nil {
                                if !set[_upload.UUID].EmitPause() {
                                    set[_upload.UUID].log(wsLogTag).WithFields(log.Fields{"act": ActPause}).Error(errors.New("pausing failed in pause message"))

                                    var resp RespWrap
                                    resp.setStatus(500, "Pausing failed in pause message.")

                                    var ulWrap UploadList
                                    ulWrap.Fill(set[_upload.UUID])

                                    resp.respWrapper(strings.ToLower(ActPause), ulWrap)
                                    if err := WsConn.WriteJSON(resp); err != nil {
                                        logger(wsLogTag).Error(err)
                                    }
                                }
                            }
                        }
                    }

                    UploadSave.Unlock()
                }
            }

        // Aborting files transferred.
        case ActAbort:
            var uploadMsg UploadOptMsg
            err = json.Unmarshal(recv, &uploadMsg)

            logger(wsLogTag).WithFields(log.Fields{"act": ActAbort}).Debug("act-abort is calling: ", string(recv))

            if err != nil {
                logger(wsLogTag).WithFields(log.Fields{"act": ActAbort}).Error(errors.New("data illegal in act-abort message: " + string(recv)))

                resp.setStatus(400, "The request data is illegal in message of act-abort: " + string(recv))
                resp.respWrapper(strings.ToLower(ActPause))
                if err := c.WriteJSON(resp); err != nil {
                    logger(wsLogTag).Error(err)
                }
            } else {
                if UploadSave.List == nil || len(UploadSave.List) == 0 {
                    logger(wsLogTag).WithFields(log.Fields{"act": ActAbort}).Error(errors.New("no job are processing in act-abort message"))

                    resp.setStatus(403, "There is no job are processing.")
                    resp.respWrapper(strings.ToLower(ActAbort))

                    if err := c.WriteJSON(resp); err != nil {
                        logger(wsLogTag).Error(err)
                    }
                } else {
                    UploadSave.Lock()

                    // Aborting all the processing tasks when opt-msg is empty.
                    if uploadMsg.Opt.List == nil {
                        for _, upload := range UploadSave.List {
                            if !upload.EmitAbort() {
                                upload.log(wsLogTag).WithFields(log.Fields{"act": ActAbort, "su": UploadSave}).Error(errors.New("abort failed in act-abort message"))

                                // Send message to websocket client.
                                var resp RespWrap
                                resp.setStatus(500, "Aborting failed in abort message.")

                                var ulWrap UploadList
                                ulWrap.Fill(upload)

                                resp.respWrapper(strings.ToLower(ActAbort), ulWrap)
                                if err := WsConn.WriteJSON(resp); err != nil {
                                    logger(wsLogTag).Error(err)
                                }
                            }
                        }
                    } else {
                        // Aborting specified processing task from uuid.
                        set := make(map[int64]*Upload)
                        for _, upload := range UploadSave.List {
                            set[upload.UUID] = upload
                        }

                        for _, _upload := range uploadMsg.Opt.List {
                            if set[_upload.UUID] != nil {
                                if !set[_upload.UUID].EmitAbort() {
                                    set[_upload.UUID].log(wsLogTag).WithFields(log.Fields{"act": ActAbort}).Error(errors.New("aborting failed in abort message"))

                                    // Send message to websocket client.
                                    var resp RespWrap
                                    resp.setStatus(500, "Aborting failed in abort message.")

                                    var ulWrap UploadList
                                    ulWrap.Fill(set[_upload.UUID])

                                    resp.respWrapper(strings.ToLower(ActAbort), ulWrap)
                                    if err := WsConn.WriteJSON(resp); err != nil {
                                        logger(wsLogTag).Error(err)
                                    }
                                }
                            }
                        }
                    }

                    UploadSave.Unlock()
                }
            }

        default:
            resp.setStatus(400, "request data illegal: " + string(recv))
            if err := c.WriteJSON(resp); err != nil {
                logger(wsLogTag).Error(err)
            }
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
func (resp *RespWrap) setStatus(code int16, optional ...string) {
    resp.Code = code

    if len(optional) == 1 {
        resp.Message = optional[0]
    } else {
        if resp.Code == 200 {
            resp.Message = "the request has succeeded."
        }
    }
}

// Response wrapper.
func (resp *RespWrap) respWrapper(args ...interface{}) {
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

func openWs() {
    defer Wg.Done()

    flag.Parse()
    http.HandleFunc("/files_trans", ws)
    logger(wsLogTag).Fatal(http.ListenAndServe(*addr, nil))
}

