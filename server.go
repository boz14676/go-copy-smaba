package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "github.com/gorilla/websocket"
    log "github.com/sirupsen/logrus"
    "io/ioutil"
    "net/http"
    "time"
)

const (
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

    // Connection global variable for websocket.
    WsConn *websocket.Conn

    Msg Message
)

// Websocket service.
func ws(w http.ResponseWriter, r *http.Request) {
    c, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Error("upgrade:", err)
        return
    }

    WsConn = c

    defer c.Close()

    var resp RespWrap
    var recv []byte

    for {

        // Get io.Reader of client message.
        _, r, err := c.NextReader()

        if err != nil {
            log.Info(err)
            _ = c.Close()
            break
        }

        // Get method which is from io.Reader of client message.
        recv, err = ioutil.ReadAll(r)
        if err != nil {
            resp.SetStatus(500, ErrMaps[5200])
            resp.Send()
        }
        if err = json.Unmarshal(recv, &Msg); err != nil {
            resp.SetStatus(400, fmt.Sprintf(ErrMaps[5201], string(recv)))
            resp.Send()
        }

        Msg.recv = recv
        Msg.Run()
    }
}

// Response wrap struct.
type RespWrap struct {
    Cmd       string      `json:"cmd,omitempty"`
    Timestamp int64       `json:"timestamp"`
    Stat      int16       `json:"code"`
    Message   string      `json:"msg"`
    Data      interface{} `json:"data,omitempty"`
}

// Response status setting.
func (resp *RespWrap) SetStatus(stat int16, message ...string) {
    resp.Stat = stat

    if len(message) > 0 && message[0] != "" {
        resp.Message = message[0]
    } else {
        if resp.Stat == 200 {
            resp.Message = ErrMaps[5300]
        } else {
            resp.Message = ErrMaps[5301]
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
        log.Error(err)
    }
}

func openWs() {
    defer Wg.Done()

    flag.Parse()
    http.HandleFunc("/files_trans", ws)
    log.Fatal(http.ListenAndServe(*addr, nil))
}

