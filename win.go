package main

import (
    "errors"
    log "github.com/sirupsen/logrus"
    "os"
    "os/exec"
    "regexp"
    "strings"
)

// Get mounted drive if it's exists.
func getMntDrive() (drive string, err error) {
    output, err := exec.Command("net", "use").Output()
    if err != nil {
        return
    }

    reg, err := regexp.Compile(`OK\s{11}([A-Z]{1}):\s{8}\\{2}` + strings.Replace(netAddr, "/", `\\`, 1) + `\\` + projectCurPath)

    if err != nil {
        return
    }

    ret := reg.FindStringSubmatch(string(output))
    if len(ret) == 2 && ret[1] != "" {
        drive = ret[1]
    }

    return
}

// Generate mounted drive which related to specified network address in Windows.
func WinMount() (drive string, err error) {
    // Return the mounted drive if it's exists.
    drive, err = getMntDrive()
    if err != nil {
        log.Error(err)
        return
    }

    if drive != "" {
        return
    }

    // Get all drives in local.
    drives, alloc := getDrives()

    if len(drives) == 26 {
        err = errors.New("too many drives in local, system cannot map any drive")
        return
    }

    // Mount a new one which is related to specified network address.
    if _, err = os.Stat(alloc); os.IsNotExist(err) {
        err = Exec2("net use " + alloc + " \\\\" + netAddr + "\\" + projectCurPath + " " + netPwd + " /user:" + netUser)

        if err != nil {
            return
        } else {
            log.Info("mount built has succeeded")

            // Rename the name of drive description.
            _ = Exec2(`reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Explorer\MountPoints2\##` + strings.Replace(netAddr, "/", "#", -1) + `" /f /v "_LabelFromReg" /t REG_SZ /d "` + projectCurPath + `"`)
        }
    } else {
        err = errors.New("mount built has failed")
        return
    }

    return
}

// Get all drives that is exists in local.
func getDrives() (r []int32, alloc string) {
    for _, drive := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
        if _, err := os.Open(string(drive) + ":\\"); err == nil {
            r = append(r, drive)
        } else {
            if alloc == "" {
                alloc = string(drive) + ":"
            }
        }
    }

    return
}
