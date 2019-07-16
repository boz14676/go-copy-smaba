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
    SourceMd5     string    `json:"source_md5" db:"source_md5"`
    SourceFile    string    `json:"origin" db:"source_filename"`
    DestFile      string    `json:"dest,omitempty"`
    BaseDestFile  string    `json:"-" db:"dest_filename"`
    TransSizePrev int64     `json:"-"`
    TransSize     int64     `json:"trxed"`
    TotalSize     int64     `json:"total" db:"files_size"`
    Status        int8      `json:"status" db:"status"`
    Enqueued      bool      `json:"-"`      // The sign is for enqueued.
    OnWatch       bool      `json:"-"`      // The sign is for heart-beat of files transfer.
    Resuming      bool      `json:"resume"` // The sign is for resuming files transferred.
    Pause         bool      `json:"-"`      // The sign is for pausing files transferred.
    Abort         bool      `json:"-"`      // The sign is for pausing files transferred.
    Stop          bool      `json:"-"`      // The sign is for stopping files transferred.
    Created       time.Time `json:"-" db:"created"`
    Updated       time.Time `json:"-" db:"updated"`
}

var (
    // Hang up is the error returned by Copy21 when emit pausing action.
    ErrHangup = errors.New("hang-up")
)

// Rewrite Write function of io.Copy to recording the processing of transfer files.
func (pt *Upload) Write(p []byte) (int, error) {
    n := len(p)
    pt.TransSize += int64(n)

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

        upload.log().WithFields(log.Fields{"size": upload.TransSize}).Debug("get remote file size succeeded")
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

        // This is only for debugging.
        // if upload.TransSize >= 70000000 && upload.Resuming == false {
        //     err = errors.New("readBytes already read 70 megabytes")
        //     return err
        // }
    }
}

func (upload *Upload) EmitWatch() {
    if upload.OnWatch == true || (upload.Status != StatusWaited && upload.Status != StatusProcessing) {
        upload.SendMsg(ActWatch, 500, 5004)
        return
    }

    upload.OnWatch = true
}

func (upload *Upload) EmitOffWatch() {
    if upload.OnWatch == true {
        upload.OnWatch = false
    }
}

func (upload *Upload) EmitPause() {
    if upload.Pause == true || upload.Status != StatusProcessing {
        upload.SendMsg(ActPause, 400, 5002)
        return
    }

    upload.Pause = true
}

func (upload *Upload) EmitAbort() {
    if upload.Abort == true || upload.Status != StatusProcessing {
        upload.SendMsg(ActPause, 400, 5003)
        return
    }

    upload.Abort = true
}

// Reset all stuff for upload task.
func (upload *Upload) Reset() {
    // Set upload status is waited.
    upload.Status = StatusWaited
    // Reset the sign for pausing.
    upload.Pause = false
    // Reset the sign for enqueued.
    upload.Enqueued = false
}

// TODO: mutex lock for upload task.
func (upload *Upload) EmitResuming() error {
    // Finding source and dest filename from database.
    if err := upload.Find(); err != nil {
        upload.SendMsg(ActResume, 500, 3008)
        return err
    }

    // Limit for condition of emitting.
    if upload.Resuming || upload.Status == StatusSucceeded || upload.Status == StatusCancelled {
        upload.SendMsg(ActResume, 500, 5001)
        return errors.New(ErrMaps[5001])
    }

    // Mark the upload task as for resuming files transferred.
    upload.Resuming = true

    if upload.Status != StatusWaited {
        upload.Reset()
    }

    // Update status of upload task in database.
    if _, err := upload.SaveStatus(); err != nil {
        upload.SendMsg(ActResume, 500, 3007)
        return err
    }

    return nil
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

// Upload task processing.
func (upload *Upload) Process() {
    defer func() {
        // Ignore pop the upload when the upload's status is hang-up.
        if upload.Status == StatusHangup {
            return
        }

        UploadSave.Lock()

        for i, _upload := range UploadSave.List {
            if _upload.UUID == upload.UUID {
                // upload.log().Debug("get rm index: ", i)
                UploadSave.List = append(UploadSave.List[:i], UploadSave.List[i+1:]...)
                // upload.log().Debug("remove rm index: ", i)
                break
            }
        }

        UploadSave.Unlock()
    }()

    var err error

    upload.log(1002).Info()

    // Mark status as processing.
    upload.Status = StatusProcessing
    if _, err = upload.SaveStatus(); err != nil {
        upload.log(3002).Error(err)
    }

    // Start copy process.
    err = upload.Copy21()

    // If files transfer is successful.
    if err == io.EOF {
        // Mark status as succeeded.
        upload.Status = StatusSucceeded

        upload.log(1003).Info()

        // Send message to websocket client.
        upload.SendMsg(ActWatch, 200, 1003)

        // Save status in database.
        if _, err = upload.SaveStatus(); err != nil {
            upload.log(3003).Error(err)
        }
    } else if err == ErrHangup {
        // If it is cancel upload task.
        if upload.Abort {
            upload.erase()
        } else {
            // Mark status as hang-up
            upload.Status = StatusHangup

            upload.log(1001).Info()

            // Send message to websocket client.
            upload.SendMsg(ActPause, 200, 1001)

            // Save status in database.
            if _, err = upload.SaveStatus(); err != nil {
                upload.log(3005).Error(err)
            }
        }

    } else {
        // Mark status as failed.
        upload.Status = StatusFailed

        upload.log(1005).Error(err)

        // Send message to websocket client.
        upload.SendMsg(ActWatch, 500, 1005)

        // Save status in database.
        if _, err = upload.SaveStatus(); err != nil {
            upload.log(3006).Error(err)
        }

        return
    }
}

func (upload *Upload) erase() {
    var err error

    // Mark status as cancelled.
    upload.Status = StatusCancelled

    upload.log(1004).Info()

    // Save status in database.
    if _, err = upload.SaveStatus(); err != nil {
        upload.log(3004).Error(err)
    }

    // Remove destination file.
    err = os.Remove(upload.DestFile)
    if err != nil {
        upload.log(2004).Error(err)

        // Send message to websocket client.
        upload.SendMsg(ActAbort, 500, 2004)

        return
    }

    // Send message to websocket client.
    upload.SendMsg(ActAbort, 200, 1004)
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

// Get transfer size of destfile
func (upload *Upload) getTransSize() (err error) {
    // Get dest filename if it's empty but base dest filename is not empty.
    if upload.DestFile == "" && upload.BaseDestFile != "" {
        if err = upload.Setup(); err != nil {
            return err
        }
    } else if upload.DestFile == "" {
        return errors.New(ErrMaps[2006])
    }

    // Get remote size of files which has transferred.
    destFileStat, err := os.Stat(upload.DestFile)
    if err != nil {
        return err
    }
    upload.TransSize = destFileStat.Size()

    upload.log().WithFields(log.Fields{"size": upload.TransSize}).Debug("get remote file size succeeded")

    return
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
        upload.log(2001).Fatal(err)
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
func (upload *Upload) log(optional ...int32) *log.Entry {
    logFields := make(log.Fields)

    if len(optional) > 0 {
        // Inject err code to logger.
        if optional[0] != 0 {
            logFields["code"] = optional[0]
            if ErrMaps[optional[0]] != "" {
                logFields["message"] = ErrMaps[optional[0]]
            }
        }
    }

    // return logger(logTag).WithFields(log.Fields{
    //     "upload": fmt.Sprintf("%+v", *upload),
    // })

    uploadMap := make(map[string]interface{})
    uploadMap["uid"] = upload.UUID
    uploadMap["source_file"] = upload.SourceFile
    uploadMap["dest_file"] = upload.DestFile

    logFields["upload"] = uploadMap

    return log.WithFields(logFields)
}

// Send message to websocket client.
func (upload *Upload) SendMsg(act string, code int16, errCode ...int32) {
    var resp RespWrap
    var msg string

    if len(errCode) > 0 && errCode[0] != 0 {
        msg = ErrMaps[errCode[0]]
    }

    resp.SetStatus(code, msg)

    var ulWrap UploadList
    ulWrap.Fill(upload)

    resp.RespWrapper(strings.ToLower(act), ulWrap)
    resp.Send()
}
