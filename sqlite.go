package main

import (
    "errors"
    "github.com/jmoiron/sqlx"
    _ "github.com/mattn/go-sqlite3"
    "strconv"
    "time"
)

const (
    StatusWaited     int8 = 0 // Status of waited.
    StatusProcessing int8 = 1 // Status of proceed.
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
    DBInstance.db, err = sqlx.Open("sqlite3", "./foo.db")
    if err != nil {
        logger(SqliteLogTag).Panic(err)
    }
}

// Store data of files trans in database.
func (upload *Upload) Store() (err error) {
    // Check if exists data by source filename.
    _ = DBInstance.db.Get(&upload.UUID, "SELECT id FROM files_trans WHERE source_md5=$1", upload.SourceMd5)
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
        "update files_trans set status=$1, updated=$3 where id=$4",
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

func (upload *Upload) List(listMsg ListMsg) (uploadList []interface{}, err error) {
    querySql := "SELECT * FROM files_trans WHERE 1=1"

    // Filter for created of time.
    if listMsg.Created != 0 {
        querySql += " AND created = \"" + time.Unix(listMsg.Created, 0).String() + "\""
    } else {
        if listMsg.StartedAt != 0 && listMsg.EndedAt != 0 {

            querySql += " AND created BETWEEN \"" +
                time.Unix(listMsg.StartedAt, 0).String() + "\" AND \"" +
                time.Unix(listMsg.EndedAt, 0).String() + "\""

        } else if listMsg.StartedAt != 0 {

            querySql += " AND created >= \"" + time.Unix(listMsg.StartedAt, 0).String() + "\""

        } else if listMsg.EndedAt != 0 {

            querySql += " AND created <= \"" + time.Unix(listMsg.EndedAt, 0).String() + "\""

        }
    }

    // Filter status.
    if listMsg.Status != -1 {
        querySql += " AND status = " + strconv.Itoa(int(listMsg.Status))
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

    rows, err := DBInstance.db.Query(querySql)
    if err != nil {
        return
    }
    defer rows.Close()

    // logrus.Debug(SafeStr(querySql))

    var _upload Upload
    for rows.Next() {
        err = rows.Scan(&_upload.UUID, &_upload.SourceFile, &_upload.DestFile, &_upload.Status, &_upload.TotalSize, &_upload.Created, &_upload.Updated)
        if err != nil {
            logger(SqliteLogTag).Error("rows scanning: ", err)
            continue
        }

        uploadList = append(uploadList, struct {
            Upload
            Created int64
            Updated int64
        }{
            Upload:  _upload,
            Created: _upload.Created.Unix(),
            Updated: _upload.Updated.Unix(),
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
        "SELECT source_filename, dest_filename FROM files_trans WHERE id=$1 LIMIT 1",
        upload.UUID,
    )
}
