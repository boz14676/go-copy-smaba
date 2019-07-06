package main

import (
    "errors"
    "fmt"
    "github.com/google/uuid"
    log "github.com/sirupsen/logrus"
    "io"
    "os"
    "os/exec"
    "os/user"
    "path/filepath"
    "runtime"
    "strings"
    "sync"
    "time"
)

const (
    // Local path.
    localPath = ".ff_sfw-tmp"

    // Network address which contains project path of foundation.
    // netAddr = "192.168.1.180/smb-storage"
    netAddr = "LAPTOP-UHA20DPJ/Users/Feelfine/smb-storage"

    // Network username.
    netUser = "smb_user"

    // Network user password.
    netPwd = "smb123"

    // Network current project path.
    projectCurPath = "smb_test"

    // Upload log tag.
    uploadLogTag = "upload"
)

// Upload struct.
type Upload struct {
    UUID       int64     `json:"uid,omitempty"`
    SourceFile string    `json:"origin"`
    DestFile   string    `json:"dest,omitempty"`
    TransSize  int64     `json:"trxed"`
    TotalSize  int64     `json:"total"`
    Status     int8      `json:"status"`
    IsEnqueued bool      `json:"-"` // The mark for enqueued.
    IsOnWatch  bool      `json:"-"` // The mark for heart-beat of files transfer.
    IsPerCmplt bool      `json:"-"` // The mark for is pre-complete.
    Created    time.Time `json:"-"`
    Updated    time.Time `json:"-"`
}

var ProcCounter int64

// Upload list struct.
type UploadList struct {
    sync.Mutex

    List []*Upload `json:"list"`
}

// Rewrite Write function of io.Copy to recording the processing of transfer files.
func (pt *Upload) Write(p []byte) (int, error) {
    n := len(p)
    pt.TransSize += int64(n)

    // log.Debug("Trans: ", humanize.Bytes(uint64(pt.TransSize)), " Total: ", humanize.Bytes(uint64(pt.TotalSize)))

    return n, nil
}

// Copy source file to destination file.
func (upload *Upload) Copy2() (nBytes int64, err error) {
    sourceFileStat, err := os.Stat(upload.SourceFile)
    if err != nil {
        return
    }

    if !sourceFileStat.Mode().IsRegular() {
        err = errors.New(fmt.Sprintf("%s is not a regular file", upload.SourceFile))

        return
    }

    // Set the total size of the source file.
    upload.TotalSize = sourceFileStat.Size()

    source, err := os.Open(upload.SourceFile)
    if err != nil {
        return
    }
    defer source.Close()

    destination, err := os.Create(upload.DestFile)
    if err != nil {
        return
    }
    defer destination.Close()

    src := io.TeeReader(source, upload)

    nBytes, err = io.Copy(destination, src)

    return
}

func (upload *Upload) EmitWatch() {
    if upload.IsOnWatch == false {
        upload.IsOnWatch = true
    }
}

func (upload *Upload) EmitOffWatch() {
    if upload.IsOnWatch == true {
        upload.IsOnWatch = false
    }
}

// Execute os command.
func Exec2(cmd string) (err error) {
    logger(uploadLogTag).Debug("Command: \"" + cmd + "\"")

    parts := strings.Fields(cmd)

    head := parts[0]
    parts = parts[1:]

    _, err = exec.Command(head, parts...).Output()

    return
}

// Mount a local mapping which is related to specified network address.
func Mount() (destDir string, err error) {
    // Get current user.
    usr, err := user.Current()
    if err != nil {
        return
    }

    destDir = usr.HomeDir + "/" + localPath

    // Make a directory from $HOME if the specified directory is not exists.
    if _, err = os.Stat(destDir); os.IsNotExist(err) {
        if err = os.Mkdir(destDir, 0755); err != nil {
            return
        }
    }

    // Make the mount if there is no mounted.
    output, err := exec.Command("sh", "-c", `if mount | grep "on `+destDir+`" > /dev/null; then echo "1"; fi`).Output()

    if strings.TrimRight(string(output), "\n") != "1" {
        err = Exec2("mount_smbfs smb://" + netUser + ":" + netPwd + "@" + netAddr + "/" + projectCurPath + " " + destDir)

        if err != nil {
            logger(uploadLogTag).Error(errors.New("mount built has failed"))
        } else {
            logger(uploadLogTag).Info("mount built has succeeded")
        }

        return
    }

    return
}

// Upload process launched for client message.
func (uploadList *UploadList) Process() {
    for _, upload := range uploadList.List {
        if upload.IsEnqueued {
            continue
        }

        // Generate destination filename.
        destFile, err := upload.genFilename(filepath.Ext(upload.SourceFile))
        checkErr(uploadLogTag, err)

        // Setup for upload struct.
        err = upload.Setup(upload.SourceFile, destFile)
        checkErr(uploadLogTag, err)

        // Store into db.
        err = upload.Store()
        checkErr(SqliteLogTag, err)

        // Push into task queue.
        Enqueue(upload)

        // Mark as is enqueued.
        upload.IsEnqueued = true

        ProcCounter++
    }
}

// Fill single upload element into upload list.
func (uploadList *UploadList) Fill(upload *Upload) {
    if len(uploadList.List) == 0 {
        uploadList.List = make([]*Upload, 1)
        uploadList.List[0] = upload
    } else {
        uploadList.List = append(uploadList.List, upload)
    }
}

// Upload task processing.
func (upload *Upload) Process() {

    defer func() {
        // TODO: It will retry 3 times for the failed job.

        ProcCounter--
    }()

    upload.log(taskLogTag).Info("the job is launched")

    // Mark as status is proceed.
    upload.Status = StatusProcessing

    // Start copy process.
    nBytes, err := upload.Copy2()

    if err != nil {
        // Mark as status is failed.
        upload.Status = StatusFailed

        upload.log(taskLogTag).Error(err)

        // Send message to websocket client.
        var resp RespWrap
        resp.setStatus(500, err.Error())

        var ulWrap UploadList
        ulWrap.Fill(upload)

        resp.respWrapper(strings.ToLower(ActWatch), ulWrap)
        if err := WsConn.WriteJSON(resp); err != nil {
            logger(wsLogTag).Error(err)
        }

        // Save status in database.
        if _, err = upload.SaveStatus(); err != nil {
            upload.log(taskLogTag).Error(err)
        }

        return
    }

    // Mark as status is succeeded.
    upload.Status = StatusSucceeded

    upload.log(taskLogTag).Info("the job is succeeded")

    // Send message to websocket client.
    var resp RespWrap
    resp.setStatus(200, "The task has been succeeded.")

    var ulWrap UploadList
    ulWrap.Fill(upload)

    resp.respWrapper(strings.ToLower(ActWatch), ulWrap)
    if err := WsConn.WriteJSON(resp); err != nil {
        logger(wsLogTag).Error(err)
    }

    // Save status in database.
    _, err = upload.SaveStatus(nBytes)

    if err != nil {
        upload.log(taskLogTag).Error(err)
    }
}

// Generate filename for writing.
func (upload *Upload) genFilename(ext string) (uuidStr string, err error) {
    _uuid, err := uuid.NewUUID()

    if err != nil {
        return
    }

    uuidStr = _uuid.String() + ext

    return
}

// Setup function for mount a local mapping which is related to specified network address.
// Os platform supported: "MacOS", "Windows".
func (upload *Upload) Setup(sourceFile string, destFile string) (err error) {
    // Mount a local mapping which is related to specified network address.
    var destDir string

    if runtime.GOOS != "windows" {
        destDir, err = Mount()
    } else {
        destDir, err = WinMount()
    }

    if err != nil {
        return
    }

    destFile = destDir + "/" + destFile

    upload.SourceFile = sourceFile
    upload.DestFile = destFile

    return
}

// Upload struct logging.
func (upload *Upload) log(optional ...string) *log.Entry {
    var logTag string

    if len(optional) > 0 {
        if optional[0] != "" {
            logTag = optional[0]
        }
    }

    return logger(logTag).WithFields(log.Fields{
        "upload": fmt.Sprintf("%+v", *upload),
    })
}
