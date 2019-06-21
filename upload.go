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
)

const (
    // Local drive for windows.
    localWindowsDrive string = "m:"

    // Local path.
    localPath = ".ff_sfw-tmp"

    // Network address which contains project path of foundation.
    netAddr = "192.168.1.180/smb-storage"

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
    Idx        int
    UUID       int64  `json:"uid,omitempty"`
    SourceFile string `json:"origin"`
    DestFile   string `json:"dest,omitempty"`
    TransSize  int64  `json:"trxed,omitempty"`
    TotalSize  int64  `json:"total"`
    Status     int8   `json:"status"`
    Cnt        int64  `json:"cnt"`
    // The mark for the watching feature.
    IsOnWatch  bool
    io.Reader
}

// Upload list struct.
type UploadList struct {
    List []Upload `json:"list"`
}

func (pt *Upload) Read(p []byte) (int, error) {
    n, err := pt.Reader.Read(p)
    pt.TotalSize += int64(n)

    return n, err
}

func (upload *Upload) Copy2() (nBytes int64, err error) {
    upload.Status = StatusProceed

    defer func() {
        if err != nil {
            upload.Status = StatusFailed
        } else {
            upload.Status = StatusSucceeded
        }
    }()

    sourceFileStat, err := os.Stat(upload.SourceFile)
    if err != nil {
        return
    }

    if !sourceFileStat.Mode().IsRegular() {
        err = errors.New(fmt.Sprintf("%s is not a regular file", upload.SourceFile))

        return
    }

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

    src := &Upload{Reader: source}

    nBytes, err = io.Copy(destination, src)

    return
}

func Exec2(cmd string) (err error) {
    fmt.Println("command: \"" + cmd + "\"")
    parts := strings.Fields(cmd)

    head := parts[0]
    parts = parts[1:]

    _, err = exec.Command(head, parts...).Output()

    return
}

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

    // Mount a local mapping which is related to specified network address.
    // make the mount if there is no monuted.
    output, err := exec.Command("sh", "-c", `if mount | grep "on `+destDir+`" > /dev/null; then echo "1"; fi`).Output()

    if strings.TrimRight(string(output), "\n") != "1" {
        err = Exec2("mount_smbfs smb://" + netUser + ":" + netPwd + "@" + netAddr + "/" + projectCurPath + " " + destDir)

        // Success logging.
        logger(uploadLogTag).Info("Mount built has succeeded.")
    }

    return
}

func (upload *Upload) EmitWatch() {
    upload.IsOnWatch = true
}

func (upload *Upload) EmitOffWatch() {
    upload.IsOnWatch = false
}

// Upload process launched for client message.
func (uploadList *UploadList) Process() {
    for i := range uploadList.List {
        upload := &uploadList.List[i]
        upload.Idx = i

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
    }

    return
}

// Upload task processing.
func (upload *Upload) Process(checked chan<- bool) {
    defer func() {
        // TODO: It will retry 3 times for the failed job.
        // Remove current element from the slice.
        UploadSave.List = append(UploadSave.List[:upload.Idx], UploadSave.List[upload.Idx+1:]...)

        // Channel of finished signal.
        checked <- true
    }()

    upload.log(taskLogTag).Info("the job is launched")

    nBytes, err := upload.Copy2()

    if err != nil {
        upload.log(taskLogTag).Error(err)

        if _, err = upload.Save(StatusFailed); err != nil {
            upload.log(taskLogTag).Error(err)
        }
    }

    upload.log(taskLogTag).Info("the job is succeeded")

    _, err = upload.Save(StatusSucceeded, nBytes)

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
// Os platform supported: "Macos", "Windows".
func (upload *Upload) Setup(sourceFile string, destFile string) (err error) {
    // Mount a local mapping which is related to specified network address.
    var destDir string

    if runtime.GOOS != "windows" {
        destDir, err = Mount()
    } else {
        // destDir, err = win_mount()
    }

    checkErr(uploadLogTag, err)

    destFile = destDir + "/" + destFile

    upload.SourceFile = sourceFile
    upload.DestFile = destFile

    return
}

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
