package main

import (
    "encoding/json"
    "errors"
    "flag"
    "github.com/gorilla/websocket"
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

    // Action for watch of heartbeat-pack.
    ActWatch = "WATCH"

    // Action for offwatch of heartbeat-pack.
    ActOffWatch = "OFFWATCH"

    // Action for list of uploaded.
    ActList = "LIST"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true
    },
}

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

// Upload message global variable.
var UploadSave UploadList

// Connection global variable for websocket.
var WsConn *websocket.Conn

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

        // Get io.Reader of client message.
        _, r, err := c.NextReader()
        checkErr(wsLogTag, err)

        // Get method which is from io.Reader of client message.
        recv, err := ioutil.ReadAll(r)

        err = json.Unmarshal(recv, &m)
        if err != nil {
            resp.setStatus(400, "request data illegal: " + string(recv))
            if err := c.WriteJSON(resp); err != nil {
                logger(wsLogTag).Error(err)
            }

            continue
        }

        // Main process.
        switch strings.ToUpper(m.Method) {

        // Process files upload.
        case ActUpload:
            var uploadMsg UploadOptMsg

            // Log for request "upload" message.
            logger(wsLogTag).Debug("act-upload is calling: ", string(recv))

            err = json.Unmarshal(recv, &uploadMsg)
            if err != nil || uploadMsg.Opt.List == nil || len(uploadMsg.Opt.List) == 0 {
                logger(wsLogTag).Error(errors.New("data illegal in message of upload: " + string(recv)))

                resp.setStatus(400, "request data illegal in message of upload: " + string(recv))
                if err := c.WriteJSON(resp); err != nil {
                    logger(wsLogTag).Error(err)
                }
            } else {
                UploadSave.Lock()

                UploadSave.List = append(UploadSave.List, uploadMsg.Opt.List...)

                UploadSave.Process()

                UploadSave.Unlock()

                resp.setStatus(200)
                resp.respWrapper(strings.ToLower(ActUpload), UploadSave)
                if err := c.WriteJSON(resp); err != nil {
                    logger(wsLogTag).Error(err)
                }
            }

        // Get list for file tasks.
        case ActList:
            var listMsg ListOptMsg
            listMsg.Opt.Init()

            logger(wsLogTag).Debug("act-list is calling: ", string(recv))

            err = json.Unmarshal(recv, &listMsg)
            if err != nil {
                logger(wsLogTag).Error(err)

                resp.setStatus(400, "request data illegal in message of list: " + string(recv))
                if err := c.WriteJSON(resp); err != nil {
                    logger(wsLogTag).Error(err)
                }

            } else {
                var upload Upload
                uploadList, err := upload.List(listMsg.Opt)
                if err != nil {
                    logger(wsLogTag).Error(err)

                    resp.setStatus(500, "response errors occurred: " + string(recv))
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

            logger(wsLogTag).Debug("act-watch is calling: ", string(recv))

            if err != nil {
                logger(wsLogTag).Error(errors.New("request data illegal in message of act-watch: " + string(recv)))

                resp.setStatus(400, "request data illegal in message of watch: " + string(recv))
                if err := c.WriteJSON(resp); err != nil {
                    logger(wsLogTag).Error(err)
                }
            } else {
                // Check global variable "uploadSave" if empty.
                if UploadSave.List == nil || len(UploadSave.List) == 0 {
                    logger(wsLogTag).Error(errors.New("no job are processing in message of act-watch"))

                    resp.setStatus(403, "no job are processing")
                    resp.respWrapper(strings.ToLower(ActWatch))

                    if err := c.WriteJSON(resp); err != nil {
                        logger(wsLogTag).Error(err)
                    }
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
            var watchMsg UploadOptMsg
            err = json.Unmarshal(recv, &watchMsg)

            logger(wsLogTag).Debug("act-off-watch is calling: ", string(recv))

            if err != nil {
                logger(wsLogTag).Error(errors.New("request data illegal in message of act-off-watch: " + string(recv)))

                resp.setStatus(400, "request data illegal in message of off-watch: " + string(recv))
                if err := c.WriteJSON(resp); err != nil {
                    logger(wsLogTag).Error(err)
                }
            } else {
                if UploadSave.List == nil || len(UploadSave.List) == 0 {
                    logger(wsLogTag).Error(errors.New("no job are processing in message of act-off-watch"))

                    resp.setStatus(403, "no job are processing")
                    resp.respWrapper(strings.ToLower(ActWatch))

                    if err := c.WriteJSON(resp); err != nil {
                        logger(wsLogTag).Error(err)
                    }
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
    Cmd       string      `json:"cmd"`
    Timestamp int64       `json:"timestamp"`
    Code      int16       `json:"code"`
    Message   string      `json:"msg"`
    Data      interface{} `json:"data"`
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

func home(w http.ResponseWriter, r *http.Request) {
    homeTemplate.Execute(w, "ws://"+r.Host+"/files_trans")
    // homeTemplate.Execute(w, "ws://localhost:333/ws")
}

func openWs() {
    defer Wg.Done()

    flag.Parse()
    http.HandleFunc("/files_trans", ws)
    http.HandleFunc("/", home)
    logger(wsLogTag).Fatal(http.ListenAndServe(*addr, nil))
}

var homeTemplate = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<script>
// Websocket.
let ws = new WebSocket("{{.}}")

// Upload message.
let uploadMsg = [
    {"cmd":"upload","opt":{"list":[{"origin":"/Users/zhangbo/Downloads/archive/cv-module.pdf"},{"origin":"/Users/zhangbo/Downloads/archive/php-comment.md"}]}},
    {"cmd":"upload","opt":{"list":[{"origin":"/Users/zhangbo/Codec/images-input/paciffic.99974.dpx"}, {"origin": "/Users/zhangbo/Downloads/archive/ProjectBriefing.pdf"}]}},
    {"cmd":"upload","opt":{"list":[{"origin":"/Volumes/Seagate/Codec/audio-input/120.ac3"}]}},

    {"cmd":"upload","opt":{"list":[
        {"origin": "/Users/zhangbo/Archive/Save/test/B-01.pdf"},
        {"origin": "/Users/zhangbo/Archive/Save/test/B-02.pdf"},
        {"origin": "/Users/zhangbo/Archive/Save/test/B-03.pdf"},
        {"origin": "/Users/zhangbo/Archive/Save/test/B-04.pdf"},
        {"origin": "/Users/zhangbo/Archive/Save/test/B-05.pdf"},
        {"origin": "/Users/zhangbo/Archive/Save/test/B-06.pdf"},
        {"origin": "/Users/zhangbo/Archive/Save/test/B-07.pdf"},
        {"origin": "/Users/zhangbo/Archive/Save/test/B-08.pdf"},
        {"origin": "/Users/zhangbo/Archive/Save/test/B-09.pdf"},
        {"origin": "/Users/zhangbo/Archive/Save/test/B-10.pdf"},
    ]}},

    {"cmd":"upload","opt":{"list":[
        {"origin": "/Users/zhangbo/Archive/Save/test/D-01.pdf"},
        {"origin": "/Users/zhangbo/Archive/Save/test/D-02.pdf"},
        {"origin": "/Users/zhangbo/Archive/Save/test/D-03.pdf"},
        {"origin": "/Users/zhangbo/Archive/Save/test/D-04.pdf"},
        {"origin": "/Users/zhangbo/Archive/Save/test/D-05.pdf"},
        {"origin": "/Users/zhangbo/Archive/Save/test/D-06.pdf"},
        {"origin": "/Users/zhangbo/Archive/Save/test/D-07.pdf"},
        {"origin": "/Users/zhangbo/Archive/Save/test/D-08.pdf"},
        {"origin": "/Users/zhangbo/Archive/Save/test/D-09.pdf"},
        {"origin": "/Users/zhangbo/Archive/Save/test/D-10.pdf"},
    ]}},

    
]

let listMsg = {"cmd": "list", "opt": {}}

let watchMsg = {
    cmd: "watch",
    opt: {}
}

let offWatchMsg = {
    cmd: "offwatch",
    opt: {}
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

ws.onopen = async function(evt) {
    // ws.send(JSON.stringify(uploadMsg[0]))
    // ws.send(JSON.stringify(uploadMsg[1]))

    // ws.send(JSON.stringify(uploadMsg[2]))

    ws.send(JSON.stringify(uploadMsg[3]))

    await sleep(2000)

    ws.send(JSON.stringify(uploadMsg[4]))

    // ws.send(JSON.stringify(watchMsg))

    // ws.send(JSON.stringify(listMsg))

    // await sleep(2000)

    // ws.send(JSON.stringify(offWatchMsg))

}
</script>
</head>

</html>
`))
