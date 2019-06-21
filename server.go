package main

import (
    "encoding/json"
    "errors"
    "flag"
    "fmt"
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
type UploadMsg struct {
    Opt UploadList `json:"opt"`
}

// Upload message variable.
var UploadSave *UploadList

// Websocket service.
func ws(w http.ResponseWriter, r *http.Request) {
    c, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        logger(wsLogTag).Error("upgrade:", err)
        return
    }

    defer c.Close()

    for {
        var m Message

        // Ticker
        Ticker(c)

        // Get io.Reader of client message.
        _, r, err := c.NextReader()
        checkErr(wsLogTag, err)

        // Get method which is from io.Reader of client message.
        recv, err := ioutil.ReadAll(r)

        err = json.Unmarshal(recv, &m)
        checkErr(wsLogTag, err)

        // Main process.
        switch strings.ToUpper(m.Method) {

        // Process files upload.
        case ActUpload:
            var uploadMsg UploadMsg

            err = json.Unmarshal(recv, &uploadMsg)
            checkErr(wsLogTag, err)

            if uploadMsg.Opt.List == nil || len(uploadMsg.Opt.List) == 0 {
                checkErr(wsLogTag, errors.New("data illegal in message of upload"))
            }

            // Inject list parameter of `opt` in upload message into `UploadSave` global variable.
            UploadSave = &uploadMsg.Opt
            UploadSave.Process()

            // fmt.Printf("UploadMsg: %+v \n", UploadSave)

            var resp RespWrap
            if len(UploadSave.List) == 0 {
                logger(wsLogTag).Error(errors.New("illegal data response for upload message"))

                resp.setStatus(500, "error data responded")
            } else {
                resp.setStatus(200)
            }

            resp.respWrapper(strings.ToLower(ActUpload), UploadSave)

            if err := c.WriteJSON(resp); err != nil {
                logger(wsLogTag).Error(err)
            }

        case ActWatch:
            var watchMsg UploadMsg
            err = json.Unmarshal(recv, &watchMsg)
            checkErr(wsLogTag, err)

            if watchMsg.Opt.List == nil || len(watchMsg.Opt.List) == 0 {
                checkErr(wsLogTag, errors.New("data illegal in message of heartbeat"))
            }

            var resp RespWrap
            if UploadSave.List == nil || len(UploadSave.List) == 0 {
                resp.setStatus(403, "no job are processing")
            }

            // log.Printf("WatchMsg: %+v \n", UploadSave)

            set := make(map[int64]*Upload)
            for i := range UploadSave.List {
                set[UploadSave.List[i].UUID] = &UploadSave.List[i]
            }

            for _, _upload := range watchMsg.Opt.List {
                upload := set[_upload.UUID]
                if upload == nil {
                    resp.setStatus(403, fmt.Sprintf("no job found by uuid %d", _upload.UUID))

                    if err := c.WriteJSON(resp); err != nil {
                        logger(wsLogTag).Error(err)
                    }
                    return
                }

                upload.EmitWatch()
            }

            resp.setStatus(200)

            if err := c.WriteJSON(resp); err != nil {
                logger(wsLogTag).Error(err)
            }

        case ActOffWatch:
            var watchMsg UploadMsg
            err = json.Unmarshal(recv, &watchMsg)
            checkErr(wsLogTag, err)

            if watchMsg.Opt.List == nil || len(watchMsg.Opt.List) == 0 {
                checkErr(wsLogTag, errors.New("data illegal in message of heartbeat"))
            }

            var resp RespWrap
            if UploadSave.List == nil || len(UploadSave.List) == 0 {
                resp.setStatus(403, "no job are processing")
            }

            // log.Printf("WatchMsg: %+v \n", UploadSave)

            set := make(map[int64]*Upload)
            for i := range UploadSave.List {
                set[UploadSave.List[i].UUID] = &UploadSave.List[i]
            }

            for _, _upload := range watchMsg.Opt.List {
                upload := set[_upload.UUID]
                if upload == nil {
                    resp.setStatus(403, fmt.Sprintf("no job found by uuid %d", _upload.UUID))

                    if err := c.WriteJSON(resp); err != nil {
                        logger(wsLogTag).Error(err)
                    }
                    return
                }

                upload.EmitOffWatch()
            }

            resp.setStatus(200)

            if err := c.WriteJSON(resp); err != nil {
                logger(wsLogTag).Error(err)
            }

        }

    }
}

// Response wrap struct.
type RespWrap struct {
    Cmd       string `json:"cmd"`
    Timestamp string `json:"timestamp"`
    Code      int16 `json:"code"`
    Messgae   string `json:"message"`
    Data      interface{} `json:"data"`
}

// Response status setting.
func (resp *RespWrap) setStatus(code int16, optional ...string) {
    resp.Code = code

    if len(optional) == 1 {
        resp.Messgae = optional[0]
    } else {
        resp.Messgae = "the request has proceed successful."
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

    resp.Timestamp = time.Now().Format("2006-01-02 15:04:05")
}

func home(w http.ResponseWriter, r *http.Request) {
    homeTemplate.Execute(w, "ws://"+r.Host+"/files_trans")
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
window.addEventListener("load", function(evt) {
    var output = document.getElementById("output");
    var input = document.getElementById("input");
    var ws;
    var print = function(message) {
        var d = document.createElement("div");
        d.innerHTML = message;
        output.appendChild(d);
    };
    document.getElementById("open").onclick = function(evt) {
        if (ws) {
            return false;
        }
        ws = new WebSocket("{{.}}");
        ws.onopen = function(evt) {
            print("OPEN");
        }
        ws.onclose = function(evt) {
            print("CLOSE");
            ws = null;
        }
        ws.onmessage = function(evt) {
            print("RESPONSE: " + evt.data);
        }
        ws.onerror = function(evt) {
            print("ERROR: " + evt.data);
        }
        return false;
    };
    document.getElementById("send").onclick = function(evt) {
        if (!ws) {
            return false;
        }
        print("SEND: " + input.value);
        ws.send(input.value);
        return false;
    };
    document.getElementById("close").onclick = function(evt) {
        if (!ws) {
            return false;
        }
        ws.close();
        return false;
    };
});
</script>
</head>
<body>
<table>
<tr><td valign="top" width="50%">
<p>Click "Open" to create a connection to the server, 
"Send" to send a message to the server and "Close" to close the connection. 
You can change the message and send multiple times.
<p>
<form>
<button id="open">Open</button>
<button id="close">Close</button>
<p><textarea style="font-size:18px;" id="input" rows="10" cols="90"></textarea>
<button id="send">Send</button>
</form>
</td><td valign="top" width="50%">
<div id="output"></div>
</td></tr></table>
</body>
</html>
`))