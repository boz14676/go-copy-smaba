package main

import (
    "fmt"
    "io"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "strings"
)

const (
    WINDOWS_DRIVE string = "m:"

)


func copy2_01(src, dst string) (int64, error) {
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

func exec_cmd_01(cmd string) {
    //fmt.Println("command: \"" + cmd + "\"")
    // splitting head => g++ parts => rest of the command
    parts := strings.Fields(cmd)

    head := parts[0]
    parts = parts[1:]

    _, err := exec.Command(head, parts...).Output()

    if err != nil {
        fmt.Printf("%s %s", err)
    }

    // fmt.Printf("%s \n", out)
}

func mount() string {
    if runtime.GOOS != "windows" {
    } else {
        if _, err := os.Stat(WINDOWS_DRIVE); os.IsNotExist(err) {
            exec_cmd_01("net use " + WINDOWS_DRIVE + " \\\\192.168.1.180\\smb-storage\\smb_test smb123 /user:smb_user")
        }

        return WINDOWS_DRIVE
    }

    return ""
}

func main() {
    fmt.Println("The script is beginning.")

    if len(os.Args) != 3 {
       fmt.Println("Please provide one command line arguments!")
       return
    }

    sourceFile := os.Args[1]
    destFile := os.Args[2]

    // mount files
    destDir := mount()

    destFile = destDir + "/" + destFile

    nBytes, err := copy2_01(filepath.FromSlash(sourceFile), destFile)
    if err != nil {
       fmt.Printf("The copy2 operation failed %q\n", err)
    } else {
       fmt.Printf("Copied %d bytes!\n", nBytes)
    }
}
