package main

import (
    "crypto/md5"
    "encoding/hex"
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

    // Buffer size of transfer files.
    bufferSize = 1000000
)

// Upload struct.
type Upload struct {
    UUID          int64     `json:"uid,omitempty" db:"id"`
    SourceMd5     string    `json:"source_md5"`
    SourceFile    string    `json:"origin" db:"source_filename"`
    DestFile      string    `json:"dest,omitempty"`
    BaseDestFile  string    `json:"-" db:"dest_filename"`
    TransSizePrev int64     `json:"-"`
    TransSize     int64     `json:"trxed"`
    TotalSize     int64     `json:"total"`
    Status        int8      `json:"status"`
    Enqueued      bool      `json:"-"`      // The sign is for enqueued.
    OnWatch       bool      `json:"-"`      // The sign is for heart-beat of files transfer.
    Resuming      bool      `json:"resume"` // The sign is for resuming files transferred.
    Pause         bool      `json:"-"`      // The sign is for pausing files transferred.
    Abort         bool      `json:"-"`      // The sign is for pausing files transferred.
    Stop          bool      `json:"-"`      // The sign is for stopping files transferred.
    Created       time.Time `json:"-"`
    Updated       time.Time `json:"-"`
}

var (
    // Process counter.
    ProcCounter int

    // Hang up is the error returned by Copy21 when emit pausing action.
    ErrHangup = errors.New("hang-up")
)

// Upload list struct.
type UploadList struct {
    sync.RWMutex

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

// Copy source file to destination file.
func (upload *Upload) Copy21() (err error) {
    source, err := os.Open(upload.SourceFile)
    if err != nil {
        return
    }
    defer source.Close()

    // Destination file handled.
    // If it's resuming files transferred.
    if upload.Resuming {
        // Get remote size of files which has transferred.
        destFileStat, err := os.Stat(upload.DestFile)
        if err != nil {
            return err
        }
        upload.TransSize = destFileStat.Size()

        logger().WithFields(log.Fields{"size": upload.TransSize}).Debug("Get remote file size succeeded.")
    }

    destination, err := os.OpenFile(upload.DestFile, os.O_RDWR|os.O_CREATE, 0666)
    if err != nil {
        return err
    }
    defer destination.Close()

    buf := make([]byte, bufferSize)

    for {
        if upload.Pause || upload.Abort {
            logger().Debug("hang-up is caught.")
            return ErrHangup
        }

        n, err := source.ReadAt(buf, upload.TransSize)
        if err != nil && err != io.EOF {
            return err
        }

        if _, err := destination.WriteAt(buf[:n], upload.TransSize); err != nil {
            return err
        }

        // Increments transfer size in each loop iteration.
        upload.TransSize += int64(n)

        // I/O operating has completed.
        if err == io.EOF {
            return err
        }

        // if upload.TransSize != upload.TotalSize {
        //     log.WithFields(log.Fields{
        //         "ReadBytes_h": humanize.Bytes(uint64(upload.TransSize)),
        //         "readBytes":   upload.TransSize,
        //     }).Info()
        // }

        // This is only for debugging.
        // if upload.TransSize >= 70000000 && upload.Resuming == false {
        //     err = errors.New("readBytes already read 70 megabytes")
        //     return err
        // }
    }
}

func (upload *Upload) EmitWatch() {
    if upload.OnWatch == false && (upload.Status == StatusWaited || upload.Status == StatusProcessing) {
        upload.OnWatch = true
    }
}

func (upload *Upload) EmitOffWatch() {
    if upload.OnWatch == true {
        upload.OnWatch = false
    }
}

func (upload *Upload) EmitPause() bool {
    if upload.Pause == false && upload.Status == StatusProcessing {
        upload.Pause = true

        return true
    }

    return false
}

func (upload *Upload) EmitAbort() bool {
    if upload.Abort == false && upload.Status == StatusProcessing {
        upload.Abort = true

        return true
    }

    return false
}

func (upload *Upload) Abandon() {
    upload.Status = StatusFailed
}

func (upload *Upload) EmitResuming(optional ...int64) (err error) {
    // If it's been called by new upload task.
    if upload.UUID == 0 && len(optional) > 0 {
        upload.UUID = optional[0]
    } else {
        // Set upload status is waited.
        upload.Status = StatusWaited
        // Reset the sign for pausing.
        upload.Pause = false
        // Reset the sign for enqueued.
        upload.Enqueued = false
    }

    // Mark the upload task as for resuming files transferred.
    upload.Resuming = true

    // Finding source and dest filename from database.
    if err = upload.Find(); err != nil {
        return
    }

    if _, err = upload.SaveStatus(); err != nil {
        return
    }

    return
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
            return
        } else {
            logger(uploadLogTag).Info("mount built has succeeded")
        }

        return
    }

    return
}

// Upload process launched for client message.
func (uploadList *UploadList) Process() (hasErr bool) {
    for _, upload := range uploadList.List {
        if upload.Enqueued || upload.Status != StatusWaited {
            continue
        }

        var err error

        // Generate destination filename.
        if upload.BaseDestFile == "" {
            err = upload.genFilename()
            if err != nil {
                upload.log(uploadLogTag).Panic(err)
            }
        }

        // Setup for upload struct.
        err = upload.Setup()
        if err != nil {
            upload.log(SqliteLogTag).Error(err)

            // Mark status as failed.
            upload.Status = StatusFailed
            if ProcCounter > 0 {
                ProcCounter--
            }

            // Send message to websocket client.
            var resp RespWrap
            resp.setStatus(500, err.Error())

            var ulWrap UploadList
            ulWrap.Fill(upload)

            resp.respWrapper(strings.ToLower(ActUpload), ulWrap)
            if err := WsConn.WriteJSON(resp); err != nil {
                logger(wsLogTag).Error(err)
            }

            hasErr = true

            continue
        }

        // Store into db if the upload task is not for resuming files transferred.
        if upload.Resuming != true {
            err = upload.Store()
            if err != nil {
                upload.log(SqliteLogTag).Error(err)

                // Mark status as failed.
                upload.Status = StatusFailed
                if ProcCounter > 0 {
                    ProcCounter--
                }

                // Send message to websocket client.
                var resp RespWrap
                resp.setStatus(500, err.Error())

                var ulWrap UploadList
                ulWrap.Fill(upload)

                resp.respWrapper(strings.ToLower(ActUpload), ulWrap)
                if err := WsConn.WriteJSON(resp); err != nil {
                    logger(wsLogTag).Error(err)
                }

                hasErr = true

                continue
            }
        }

        // Push into task queue.
        Enqueue(upload)

        // Mark as enqueued.
        upload.Enqueued = true
    }

    return
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
        if upload.Status != StatusHangup && ProcCounter > 0 {
            ProcCounter--
        }
    }()

    var err error

    upload.log(taskLogTag).Info("the job is launched")

    // Mark status as processing.
    upload.Status = StatusProcessing
    if _, err = upload.SaveStatus(); err != nil {
        upload.log(taskLogTag).Error(err)
    }

    // Start copy process.
    err = upload.Copy21()

    // If files transfer is successful.
    if err == io.EOF {
        // Mark status as succeeded.
        upload.Status = StatusSucceeded
        // Stop watching for files transfer.
        upload.EmitOffWatch()

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
        if _, err = upload.SaveStatus(); err != nil {
            upload.log(taskLogTag).Error(err)
        }
    } else if err == ErrHangup {
        // If it is cancel upload task.
        if upload.Abort {
            // Mark status as cancelled.
            upload.Status = StatusCancelled
            // Stop watching for files transfer.
            upload.EmitOffWatch()

            upload.log(taskLogTag).Info("the job is cancelled.")

            // Save status in database.
            if _, err = upload.SaveStatus(); err != nil {
                upload.log(taskLogTag).Error(err)
            }

            // Remove destination file.
            err = os.Remove(upload.DestFile)
            if err != nil {
                upload.log(taskLogTag).Error(err)

                // Send message to websocket client.
                var resp RespWrap
                resp.setStatus(500, err.Error())

                var ulWrap UploadList
                ulWrap.Fill(upload)

                resp.respWrapper(strings.ToLower(ActAbort), ulWrap)
                if err := WsConn.WriteJSON(resp); err != nil {
                    logger(wsLogTag).Error(err)
                }

                return
            }

            // Send message to websocket client.
            var resp RespWrap
            resp.setStatus(200, "The job is removed.")

            var ulWrap UploadList
            ulWrap.Fill(upload)

            resp.respWrapper(strings.ToLower(ActAbort), ulWrap)
            if err := WsConn.WriteJSON(resp); err != nil {
                logger(wsLogTag).Error(err)
            }
        } else {
            // Mark status as hang-up
            upload.Status = StatusHangup
            // Stop watching for files transfer.
            upload.EmitOffWatch()

            upload.log(taskLogTag).Info("the job is hang-up")

            // Send message to websocket client.
            var resp RespWrap
            resp.setStatus(200, "The job is hang-up.")

            var ulWrap UploadList
            ulWrap.Fill(upload)

            resp.respWrapper(strings.ToLower(ActPause), ulWrap)
            if err := WsConn.WriteJSON(resp); err != nil {
                logger(wsLogTag).Error(err)
            }

            // Save status in database.
            if _, err = upload.SaveStatus(); err != nil {
                upload.log(taskLogTag).Error(err)
            }
        }

    } else {
        // Mark status as failed.
        upload.Status = StatusFailed
        // Stop watching for files transfer.
        upload.EmitOffWatch()

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
}

// Generate filename for writing.
func (upload *Upload) genFilename() (err error) {
    _uuid, err := uuid.NewUUID()
    if err != nil {
        return
    }

    upload.BaseDestFile = _uuid.String() + filepath.Ext(upload.SourceFile)

    return
}

// Get base dest file name.
func (upload *Upload) getBaseDestFile() string {
    if upload.BaseDestFile != "" {
        return upload.BaseDestFile
    } else {
        upload.BaseDestFile = filepath.Base(upload.DestFile)
        return upload.BaseDestFile
    }
}

// Get base dest file name.
func (upload *Upload) getSourceMd5() (err error) {
    f, err := os.Open(upload.SourceFile)
    if err != nil {
        return err
    }

    defer f.Close()

    h := md5.New()
    if _, err = io.Copy(h, f); err != nil {
        return err
    }

    // Get the 16 bytes hash
    // Convert the bytes to a string
    hashInBytes := h.Sum(nil)
    upload.SourceMd5 = hex.EncodeToString(hashInBytes)

    return err
}

// Setup function for mount a local mapping which is related to specified network address.
// Os platform supported: "MacOS", "Windows".
func (upload *Upload) Setup() (err error) {
    // Mount a local mapping which is related to specified network address.
    var destDir string

    if runtime.GOOS != "windows" {
        destDir, err = Mount()
    } else {
        destDir, err = WinMount()
    }

    // Throw Panic if mounting to local mapping has failed.
    if err != nil {
        upload.log(uploadLogTag).Panic(err)
    }

    // Source file stat handled.
    sourceFileStat, err := os.Stat(upload.SourceFile)
    if err != nil {
        return
    }

    if !sourceFileStat.Mode().IsRegular() {
        err = fmt.Errorf("%s is not a regular file", upload.SourceFile)
        return
    }

    // Set md5 of source file in upload task.
    if err = upload.getSourceMd5(); err != nil {
        return
    }
    // Set the total size of the source file.
    upload.TotalSize = sourceFileStat.Size()
    // Set full path of dest file.
    upload.DestFile = destDir + "/" + upload.BaseDestFile

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
