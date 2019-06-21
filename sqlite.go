package main

import (
    "database/sql"
    "errors"
    "fmt"
    _ "github.com/mattn/go-sqlite3"
    "time"
)

const (
    StatusWaited    int8 = 0 // Status of waited.
    StatusProceed   int8 = 1 // Status of proceed.
    StatusSucceeded int8 = 2 // Status of succeeded.
    StatusFailed    int8 = 3 // Status of failed.

    // Upload log tag.
    SqliteLogTag string = "sqlite"
)

// Store data of files trans in database.
func (upload *Upload) Store() (err error) {
    db, err := sql.Open("sqlite3", "./foo.db")
    if err != nil {
        return
    }

    stmt, err := db.Prepare("INSERT INTO files_trans(source_filename, dest_filename, created, updated) values(?,?,?,?)")
    if err != nil {
        return
    }

    res, err := stmt.Exec(upload.SourceFile, upload.DestFile, time.Now(), time.Now())
    if err != nil {
        return
    }

    id, err := res.LastInsertId()
    if err != nil {
        return
    }

    if id == 0 {
        err = errors.New("insert data to db has failed")
        return
    }

    // Set UUID into upload-object.
    upload.UUID = id

    return
}

func (upload *Upload) Save(status int8, optional ...int64) (affect int64, err error) {
    db, err := sql.Open("sqlite3", "./foo.db")
    if err != nil {
        return
    }

    var nBytes int64 = 0

    if len(optional) > 0 {
        nBytes = optional[0]
    }

    stmt, err := db.Prepare("update files_trans set status=? and files_trans_size=? where id=?")
    if err != nil {
        return
    }

    res, err := stmt.Exec(status, nBytes, upload.UUID)
    if err != nil {
        return
    }

    affect, err = res.RowsAffected()
    if err != nil {
        return
    }

    if affect == 0 {
        err = errors.New("save upload in db has failed")
        return
    }

    return
}

func test() {
    db, err := sql.Open("sqlite3", "./foo.db")
    checkErr2(err)

    // 插入数据
    stmt, err := db.Prepare("INSERT INTO userinfo(username, department, created) values(?,?,?)")
    checkErr2(err)

    res, err := stmt.Exec("astaxie", "研发部门", "2012-12-09")
    checkErr2(err)

    id, err := res.LastInsertId()
    checkErr2(err)

    fmt.Println(id)
    // 更新数据
    stmt, err = db.Prepare("update userinfo set username=? where uid=?")
    checkErr2(err)

    res, err = stmt.Exec("astaxieupdate", id)
    checkErr2(err)

    affect, err := res.RowsAffected()
    checkErr2(err)

    fmt.Println(affect)

    // 查询数据
    rows, err := db.Query("SELECT * FROM userinfo")
    checkErr2(err)

    for rows.Next() {
        var uid int
        var username string
        var department string
        var created time.Time
        err = rows.Scan(&uid, &username, &department, &created)
        checkErr2(err)
        fmt.Println(uid)
        fmt.Println(username)
        fmt.Println(department)
        fmt.Println(created)
    }

    // 删除数据
    stmt, err = db.Prepare("delete from userinfo where uid=?")
    checkErr2(err)

    res, err = stmt.Exec(id)
    checkErr2(err)

    affect, err = res.RowsAffected()
    checkErr2(err)

    fmt.Println(affect)

    db.Close()

}

func checkErr2(err error) {
    if err != nil {
        panic(err)
    }
}
