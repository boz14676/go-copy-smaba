package main

import (
    "encoding/json"
    "fmt"
    log "github.com/sirupsen/logrus"
    "strings"
)

// Client message.
type Message struct {
    Method string `json:"cmd"`
    recv   []byte
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

func (msg *Message) log(code int32, tf ...interface{}) *log.Entry {
    logFields := log.Fields{
        "code": code,
        "act":  msg.Method,
    }

    if ErrMaps[code] != "" {
        if len(tf) > 0 {
            logFields["message"] = fmt.Sprintf(ErrMaps[code], tf...)
        } else {
            // Access and illegal request logging.
            if code == 5100 || code == 5101 {
                logFields["message"] = fmt.Sprintf(ErrMaps[code], strings.ToLower(msg.Method), msg.recv)
            } else {
                logFields["message"] = fmt.Sprintf(ErrMaps[code], strings.ToLower(msg.Method))
            }
        }
    }

    return log.WithFields(logFields)
}

func (msg *Message) Send(stat int16, code int32, tf ...interface{}) {
    var resp RespWrap
    var message string

    if ErrMaps[code] != "" {
        if len(tf) > 0 {
            message = fmt.Sprintf(ErrMaps[code], tf...)
        } else {
            // Access and illegal request logging.
            if code == 5100 || code == 5101 {
                message = fmt.Sprintf(ErrMaps[code], strings.ToLower(msg.Method), msg.recv)
            } else {
                message = fmt.Sprintf(ErrMaps[code], strings.ToLower(msg.Method))
            }
        }
    }

    resp.SetStatus(stat, message)
    resp.RespWrapper(strings.ToLower(msg.Method))
    resp.Send()
}

func (msg *Message) Run() {
    var resp RespWrap

    // Main process.
    switch strings.ToUpper(msg.Method) {

    // Process files upload.
    case ActUpload:
        msg.Upload()

    // Resuming files transferred.
    case ActResume:
        msg.Resume()

    // Get list for file tasks.
    case ActList:
        msg.List()

    // Watching for processing of files transfer.
    case ActWatch:
        msg.Watch()

    // Stop watching for processing of files transfer.
    case ActOffWatch:
        msg.OffWatch()

    // Pausing files transferred.
    case ActPause:
        msg.Pause()

    // Aborting files transferred.
    case ActAbort:
        msg.Abort()

    default:
        resp.SetStatus(400, "request data illegal: "+string(msg.recv))
        resp.Send()
    }
}

func (msg *Message) Upload() {
    var uploadMsg UploadMsg

    // Access logging.
    msg.log(5100).Info()

    if err := json.Unmarshal(msg.recv, &uploadMsg); err != nil || uploadMsg.Opt.List == nil {
        // Illegal request handled.
        msg.log(5101).Error()
        msg.Send(400, 5101)
    } else {
        UploadSave.Append(uploadMsg.Opt.List)
        UploadSave.Process()
    }
}

func (msg *Message) Resume() {
    var uploadMsg UploadMsg

    // Access logging.
    msg.log(5100).Info()

    if err := json.Unmarshal(msg.recv, &uploadMsg); err != nil || uploadMsg.Opt.List == nil {
        // Illegal request handled.
        msg.log(5101).Error()
        msg.Send(400, 5101)
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
                    UploadSave.FillSafe(upload)
                }
            }
        }

        UploadSave.Process()
    }
}

func (msg *Message) List() {
    var resp RespWrap

    var listMsg ListMsg
    listMsg.Opt.Init()

    // Access logging.
    msg.log(5100).Info()

    if err := json.Unmarshal(msg.recv, &listMsg); err != nil {
        msg.log(5101).Error(err)
        msg.Send(400, 5101)
    } else {
        var upload Upload
        uploadList, err := upload.List(listMsg.Opt)
        if err != nil {
            msg.log(5102, msg.Method).Error()
            msg.Send(500, 5102)
        } else {
            var listResp ListRespMsg
            listResp.List = uploadList

            resp.SetStatus(200)
            resp.RespWrapper(strings.ToLower(ActList), listResp)
            resp.Send()
        }
    }
}

func (msg *Message) Watch() {
    var watchMsg UploadMsg

    // Access logging.
    msg.log(5100).Info()

    if err := json.Unmarshal(msg.recv, &watchMsg); err != nil {
        // Illegal request handled.
        msg.log(5101).Error()
        msg.Send(400, 5101)
    } else {
        // Check global variable "uploadSave" if empty.
        if len(UploadSave.List) == 0 {
            msg.log(5103).Error()
            msg.Send(403, 5103)
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
}

func (msg *Message) OffWatch() {
    var watchMsg UploadMsg

    // Access logging.
    msg.log(5100).Info()

    if err := json.Unmarshal(msg.recv, &watchMsg); err != nil {
        // Illegal request handled.
        msg.log(5101).Error()
        msg.Send(400, 5101)
    } else {
        if len(UploadSave.List) == 0 {
            msg.log(5103).Error()
            msg.Send(403, 5103)
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
}

func (msg *Message) Pause() {
    var uploadMsg UploadMsg

    // Access logging.
    msg.log(5100).Info()

    if err := json.Unmarshal(msg.recv, &uploadMsg); err != nil {
        // Illegal request handled.
        msg.log(5101).Error()
        msg.Send(400, 5101)
    } else {
        if len(UploadSave.List) == 0 {
            msg.log(5103).Error()
            msg.Send(403, 5103)
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
}

func (msg *Message) Abort() {
    var uploadMsg UploadMsg

    // Access logging.
    msg.log(5100).Info()

    if err := json.Unmarshal(msg.recv, &uploadMsg); err != nil {
        // Illegal request handled.
        msg.log(5101).Error()
        msg.Send(400, 5101)
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
}
