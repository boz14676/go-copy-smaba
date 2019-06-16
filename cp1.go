package main

import (
    "fmt"
    "io"
    "log"
    "os"
    "os/exec"
    "os/user"
    "runtime"
    "strings"
)

const (
    /**
     * Local drive for windows.
     */
    LOCAL_WINDOWS_DRIVE string = "m:"

    /**
     * Local path.
     */
    LOCAL_PATH = ".ff_sfw-tmp"

    /**
     * Network address which contains project path of foundation.
     */
    NET_ADDR = "DESKTOP-KGHP3M7/smb-storage"

    /**
     * Network username.
     */
    NET_USER = "share_user"

    /**
     * Network user password.
     */
    NET_PWD = "123456"

    /**
     * Network current project path.
     */
    PROJECT_CUR_PATH = "smb_test"
)

func copy2(src, dst string) (int64, error) {
    sourceFileStat, err := os.Stat(src)
    if err != nil {
        return 0, err
    }

    if !sourceFileStat.Mode().IsRegular() {
        return 0, fmt.Errorf("%s is not a regular file", src)
    }

    source, err := os.Open(src)
    if err != nil {
        return 0, err
    }
    defer source.Close()

    destination, err := os.Create(dst)
    if err != nil {
        return 0, err
    }
    defer destination.Close()

    nBytes, err := io.Copy(destination, source)
    return nBytes, err
}

func exec2(cmd string) (err error) {
    // fmt.Println("command: \"" + cmd + "\"")
    parts := strings.Fields(cmd)

    head := parts[0]
    parts = parts[1:]

    _, err = exec.Command(head, parts...).Output()

    return
}

func mount() (destDir string, err error) {

    /**
     * Get current user
     */
    usr, err := user.Current()
    if err != nil {
        return
    }

    destDir = usr.HomeDir + "/" + LOCAL_PATH

    /**
     * Mkdir from $HOME if the specified directory is not exists.
     */
    if _, err = os.Stat(destDir); os.IsNotExist(err) {
        if err = os.Mkdir(destDir, 0755); err != nil {
            return
        }
    }

    /**
     * Mount a local mapping which is related to specified network address.
     */
    err = exec2("mount_smbfs smb://" + NET_USER + ":" + NET_PWD + "@" + NET_ADDR + "/" + PROJECT_CUR_PATH + " " + destDir)

    return
}

func main() {
    /**
     * Initialized.
     */
    log.Println("The cp1 script is beginning.")

    if len(os.Args) != 3 {
        log.Println("Please provide one command line arguments!")
        return
    }

    sourceFile := os.Args[1]
    destFile := os.Args[2]

    /**
     * Mount a local mapping which is related to specified network address.
     */

    var destDir string
    var err error

    if runtime.GOOS != "windows" {
        destDir, err = mount()
    } else {
        destDir, err = win_mount()
    }

    if err != nil {
        log.Fatal(err)
    }

    /**
     * Copy files.
     */
    destFile = destDir + "/" + destFile

    nBytes, err := copy2(sourceFile, destFile)

    if err != nil {
        log.Printf("The copy2 operation failed %q\n", err)
    } else {
        log.Printf("Copied %d bytes!\n", nBytes)
    }
}
