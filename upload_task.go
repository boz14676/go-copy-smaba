package main

const (
    // Timeout for each task.
    timeout = 30

    // Upload task log tag.
    taskLogTag = "upload_task"
)

var JobChan = make(chan *Upload, 1000) // Global channel queue.

// Monitor queues task.
func Monitor() {
    defer Wg.Done()

    for {
        select {
        case job := <-JobChan:
            job.Process()
        }
    }

}

func Enqueue(upload *Upload) {
    // Enqueue
    JobChan <- upload
}

// Timeout mechanism.
/*func WaitTimeout(timeout time.Duration) bool {
    ch := make(chan struct{})
    go func() {
        Wg.Wait()
        close(ch)
    }()
    select {
    case <-ch:
        return true
    case <-time.After(timeout):
        return false
    }
}*/
