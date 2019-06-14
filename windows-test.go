package main

import (
    "fmt"
    "github.com/syyongx/php2go"
    "io"
    "log"
    "os"
    "os/exec"
    "os/user"
    "runtime"
    "strings"
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
    // fmt.Println("command: \"" + cmd + "\"")
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
    usr, err := user.Current()
    if err != nil {
        log.Fatal(err)
    }

    destDir := usr.HomeDir + "/.ff_sfw-tmp"
    if ! php2go.FileExists(destDir) {
        err = os.Mkdir(destDir, 0755)
        if err != nil {
            log.Fatal(err)
        }
    }

    if runtime.GOOS != "windows" {
        exec_cmd_01("mount_smbfs smb://smb_user:smb123@192.168.1.180/smb-storage " + destDir)
    } else {
        exec_cmd_01("net use e: \\\\192.168.1.180\\smb-storage\\smb_test smb123 /user:smb_user" + destDir)
    }

    return destDir
}

func main() {
    fmt.Println("\\\\")
    os.Exit(3)

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

    nBytes, err := copy2_01(sourceFile, destFile)
    if err != nil {
        fmt.Printf("The copy2 operation failed %q\n", err)
    } else {
        fmt.Printf("Copied %d bytes!\n", nBytes)
    }
}
