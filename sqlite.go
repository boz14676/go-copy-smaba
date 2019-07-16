package main

import (
    "errors"
    "github.com/jmoiron/sqlx"
    _ "github.com/mattn/go-sqlite3"
    log "github.com/sirupsen/logrus"
    "strconv"
    "time"
)

const (
    StatusWaited     int8 = 0 // Status of waited.
    StatusProcessing int8 = 1 // Status of processing.
    StatusSucceeded  int8 = 2 // Status of succeeded.
    StatusFailed     int8 = 3 // Status of failed.
    StatusHangup     int8 = 5 // Status of hang-up.
    StatusCancelled  int8 = 6 // Status of cancel.

    // Upload log tag.
    SqliteLogTag string = "sqlite"
)

// Database struct.
type DB struct {
    db *sqlx.DB
}

// Database instance.
var DBInstance DB

// Initialized: Open database file for db instance.
func init() {
    var err error
    DBInstance.db, err = sqlx.Open("sqlite3", "./go-copy-samba.db")
    if err != nil {
        log.Fatal(err)
    }
}

// Store data of files trans in database.
func (upload *Upload) Store() (err error) {
    // Check if exists data by source filename.
    _ = DBInstance.db.Get(
        &upload.UUID,
        "SELECT id FROM files_trans WHERE status <> $1 AND status <> $2 AND source_md5=$3",
        StatusFailed,
        StatusCancelled,
        upload.SourceMd5,
    )
    if upload.UUID != 0 {
        return errors.New("repeated request for files transfer tasks")
    }

    // Insert new data to db.
    res, err := DBInstance.db.Exec(
        "INSERT INTO files_trans(source_md5, source_filename, dest_filename, files_size, created, updated) values($1, $2, $3, $4, $5, $6)",
        upload.SourceMd5, upload.SourceFile, upload.getBaseDestFile(), upload.TotalSize, time.Now(), time.Now(),
    )
    if err != nil {
        return
    }

    // Set UUID into upload-object.
    upload.UUID, err = res.LastInsertId()
    if err != nil {
        return
    }

    return
}

func (upload *Upload) SaveStatus() (affect int64, err error) {
    // Refresh task status of data in db.
    res, err := DBInstance.db.Exec(
        "update files_trans set status=$1, updated=$2 where id=$3",
        upload.Status, time.Now(), upload.UUID,
    )
    if err != nil {
        return
    }

    affect, err = res.RowsAffected()

    if affect == 0 {
        err = errors.New("save upload in db has failed")
        return
    }

    return
}

func (upload Upload) List(listMsg ListWrap) (uploadList []interface{}, err error) {
    querySql := "SELECT * FROM files_trans WHERE 1=1"

    // var c CNTR
    var querySlice []interface{}

    // Filter for created of time.
    if listMsg.Created != 0 {

        querySql += " AND created = ?"
        querySlice = append(querySlice, time.Unix(listMsg.Created, 0).String())

    } else {
        if listMsg.StartedAt != 0 && listMsg.EndedAt != 0 {

            querySql += " AND created BETWEEN ?" + " AND ?"
            querySlice = append(querySlice, time.Unix(listMsg.StartedAt, 0).String(), time.Unix(listMsg.EndedAt, 0).String())

        } else if listMsg.StartedAt != 0 {

            querySql += " AND created >= ?"
            querySlice = append(querySlice, time.Unix(listMsg.StartedAt, 0).String())

        } else if listMsg.EndedAt != 0 {

            querySql += " AND created <= ?"
            querySlice = append(querySlice, time.Unix(listMsg.EndedAt, 0).String())

        }
    }

    // Filter status.
    if listMsg.Status != -1 {
        querySql += " AND status = ?"
        querySlice = append(querySlice, strconv.Itoa(int(listMsg.Status)))
    }

    // Sort sequence.
    if listMsg.Sort != "" {
        querySql += " ORDER BY id " + listMsg.Sort
    }

    // Limit the records of returned.
    if listMsg.Offset != -1 {
        var limit string

        if listMsg.Limit != -1 {
            limit = strconv.Itoa(int(listMsg.Limit))
        } else {
            limit = "20"
        }

        querySql += " LIMIT " + limit + " OFFSET " + strconv.Itoa(int(listMsg.Offset))
    }

    rows, err := DBInstance.db.Queryx(querySql, querySlice...)
    if err != nil {
        return
    }
    defer rows.Close()

    log.Debug("Debugging query: ", querySql + " " , querySlice)

    for rows.Next() {
        err = rows.StructScan(&upload)
        if err != nil {
            log.Error("rows scanning: ", err)
            continue
        }

        // Get transfer size if upload status is in requirement list.
        if upload.Status == StatusHangup || upload.Status == StatusFailed || upload.Status == StatusProcessing {
            if err := upload.getTransSize(); err != nil {
                upload.log(2005).Error()
            }
        } else {
            upload.TransSize = upload.TotalSize
        }

        uploadList = append(uploadList, struct {
            Upload
            Created int64
            Updated int64
        }{
            Upload:  upload,
            Created: upload.Created.Unix(),
            Updated: upload.Updated.Unix(),
        })
    }

    return
}

// Finding destination file name from database.
func (upload *Upload) FindBySourceFile() (err error) {
    err = DBInstance.db.Get(&upload.UUID, "SELECT id FROM files_trans WHERE source_filename=$1", upload.SourceFile)
    if err != nil {
        return
    }

    return
}

// Finding destination file name from database.
func (upload *Upload) Find() (err error) {
    if upload.UUID == 0 {
        return errors.New("primary id in upload is illegal")
    }

    return DBInstance.db.Get(
        upload,
        "SELECT status, source_filename, dest_filename FROM files_trans WHERE id=$1 LIMIT 1",
        upload.UUID,
    )
}
