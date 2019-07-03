package main

import (
	"errors"
	"os"
)

func init() {

}

func WinMount() (drive string, err error) {
	drives, alloc := getDrives()

	if len(drives) >= 26 {
		err = errors.New("too many drives in local, system cannot map any drive")
		return
	}

	// TODO: Replace duplicated default windows local drive if it is necessary.

	if _, err = os.Stat(alloc); os.IsNotExist(err) {
		err = Exec2("net use " + alloc + " \\\\" + netAddr + "\\" + projectCurPath + " " + netUser + " /user:" + netPwd)

		if err != nil {
			return
		}
	}

	return alloc, nil
}

func getDrives() (r []string, alloc string) {
	for _, drive := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		_, err := os.Open(string(drive) + ":\\")
		if err == nil {
			r = append(r, string(drive))
		} else {
			if alloc == "" {
				alloc = string(drive) + ":"
			}
		}
	}

	return
}
