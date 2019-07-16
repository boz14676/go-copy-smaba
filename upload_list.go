package main

import (
    "sync"
)

// Upload list struct.
type UploadList struct {
    sync.RWMutex

    List []*Upload `json:"list"`
}

// Upload process launched for client message.
func (uploadList *UploadList) Process() {
    for _, upload := range uploadList.List {
        if upload.Enqueued || upload.Status != StatusWaited {
            continue
        }

        var err error

        // Generate destination filename.
        if upload.BaseDestFile == "" {
            if err = upload.genFilename(); err != nil {
                upload.log(2002).Fatal(err)
            }
        }

        // Setup for upload task.
        if err = upload.Setup(); err != nil {
            upload.log(2003).Error(err)

            // Mark status as failed.
            upload.Status = StatusFailed

            // Send message to websocket client.
            upload.SendMsg(ActUpload, 500, 2003)

            continue
        }

        // Store the upload task into database if it's not for resuming files transferred.
        if upload.Resuming != true {
            if err = upload.Store(); err != nil {
                upload.log(3001).Error(err)

                // Mark status as failed.
                upload.Status = StatusFailed

                // Send message to websocket client.
                upload.SendMsg(ActUpload, 500, 3001)

                continue
            }
        }

        // Push into task queue.
        Enqueue(upload)

        // Mark as enqueued.
        upload.Enqueued = true

        // Send message to websocket client.
        upload.SendMsg(ActUpload, 200)
    }

    // Safely reject upload list.
    uploadList.RejectSafe()
}

// Safely appended to upload list.
func (ul *UploadList) append(_ul []*Upload) {
    ul.Lock()
    ul.List = append(ul.List, _ul...)
    ul.Unlock()
}

// Fill single upload element into upload list.
func (uploadList *UploadList) Fill(upload *Upload) {
    if len(uploadList.List) == 0 {
        uploadList.List = make([]*Upload, 1)
        uploadList.List[0] = upload
    } else {
        uploadList.List = append(uploadList.List, upload)
    }
}

// Safe wrapper for reject illegal upload task.
func (ul *UploadList) RejectSafe() {
    ul.Lock()
    ul.Reject()
    ul.Unlock()
}

// Delete illegal upload task.
func (ul *UploadList) Reject() {
    for i, upload := range UploadSave.List {
        if upload.Status == StatusFailed || upload.Status == StatusSucceeded || upload.Status == StatusCancelled {
            UploadSave.List = append(UploadSave.List[:i], UploadSave.List[i+1:]...)
            ul.Reject()
            break
        }
    }
}
