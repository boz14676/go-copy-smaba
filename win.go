package main

import (
	"errors"
	"os"
	"strings"
)

func init() {

}

func WinMount() (alloc string, err error) {
	drives, alloc := getDrives()

	if len(drives) == 26 {
		err = errors.New("too many drives in local, system cannot map any drive")
		return
	}

	if _, err = os.Stat(alloc); os.IsNotExist(err) {
		err = Exec2("net use " + alloc + " \\\\" + netAddr + "\\" + projectCurPath + " " + netUser + " /user:" + netPwd)

		if err != nil {
			logger(uploadLogTag).Error(errors.New("mount built has failed"))
		} else {
			logger(uploadLogTag).Info("mount built has succeeded")
		}

		// Rename the name of drive description.
		Exec2(`reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Explorer\MountPoints2\##` + strings.Replace(netAddr, "/", "#", -1) + `" /f /v "_LabelFromReg" /t REG_SZ /d "` + projectCurPath + `"`)
	}

	return
}

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
