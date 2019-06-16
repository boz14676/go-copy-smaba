package main

import (
    "errors"
    "os"
)

func win_mount() (drive string, err error) {
    if drives := get_drives(); len(drives) >= 24 {
        err = errors.New("System cannot map any drive, too many drives in local.")
        return
    }

    // TODO: Replace duplicated default windows local dirve if it is necessary.

    if _, err := os.Stat(LOCAL_WINDOWS_DRIVE); os.IsNotExist(err) {
        err = exec2("net use " + LOCAL_WINDOWS_DRIVE + " \\\\" + NET_ADDR + "\\" + PROJECT_CUR_PATH + " " + NET_USER + " /user:" + NET_PWD)

        if err != nil {
            return
        }
    }

    return LOCAL_WINDOWS_DRIVE, nil
}

func get_drives() (r []string){
    for _, drive := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ"{
        _, err := os.Open(string(drive)+":\\")
        if err == nil {
            r = append(r, string(drive))
        }
    }
    return r
}