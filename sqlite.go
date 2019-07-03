package main

import (
    "database/sql"
    "errors"
    _ "github.com/mattn/go-sqlite3"
    "regexp"
    "strconv"
    "strings"
    "time"
)

const (
    StatusWaited     int8 = 0 // Status of waited.
    StatusProcessing int8 = 1 // Status of proceed.
    StatusSucceeded  int8 = 2 // Status of succeeded.
    StatusFailed     int8 = 3 // Status of failed.

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

func SafeStr(s string) string {
    chars := []string{"]", "^", "\\\\", "[", ".", "(", ")"}
    r := strings.Join(chars, "")
    re := regexp.MustCompile("[" + r + "]+")
    s = re.ReplaceAllString(s, "")

    return s
}

func (upload *Upload) SaveStatus(optional ...int64) (affect int64, err error) {
    db, err := sql.Open("sqlite3", "./foo.db")
    if err != nil {
        return
    }

    var nBytes int64

    if len(optional) > 0 {
        nBytes = optional[0]
    }

    stmt, err := db.Prepare("update files_trans set status=?, files_trans_size=?, updated=? where id=?")
    if err != nil {
        return
    }

    res, err := stmt.Exec(upload.Status, nBytes, time.Now(), upload.UUID)
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

func (upload *Upload) List(listMsg ListMsg) (uploadList []interface{}, err error) {
    db, err := sql.Open("sqlite3", "./foo.db")
    if err != nil {
        return
    }

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

    rows, err := db.Query(SafeStr(querySql))
    if err != nil {
        return
    }

    // logrus.Debug(SafeStr(querySql))

    for rows.Next() {
        err = rows.Scan(&upload.UUID, &upload.SourceFile, &upload.DestFile, &upload.Status, &upload.TotalSize, &upload.Created, &upload.Updated)
        if err != nil {
            logger(SqliteLogTag).Error("rows scanning: ", err)
            continue
        }

        uploadList = append(uploadList, struct {
            *Upload
            Created int64
            Updated int64
        }{
            Upload: upload,
            Created: upload.Created.Unix(),
            Updated: upload.Updated.Unix(),
        })
    }

    return
}
