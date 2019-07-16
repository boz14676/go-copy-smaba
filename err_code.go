package main

var ErrMaps = make(map[int32]string)

func init() {
    // Info message.
    ErrMaps[1001] = "the job is hang-up"
    ErrMaps[1002] = "the job is launched"
    ErrMaps[1003] = "the job is succeeded"
    ErrMaps[1004] = "the job is cancelled"
    ErrMaps[1005] = "the job is failed"

    // Common error message.
    ErrMaps[2001] = "mounted has failed"
    ErrMaps[2002] = "generate destination filename has failed"
    ErrMaps[2003] = "system setup has failed"
    ErrMaps[2004] = "removing files has failed"
    ErrMaps[2005] = "get files transferred size has failed"
    ErrMaps[2006] = "there is no dest filename when get files transferred size"

    // Database error message.
    ErrMaps[3001] = "db: store new task has failed"
    ErrMaps[3002] = "db: set task status as processing has failed"
    ErrMaps[3003] = "db: set task status as succeeded has failed"
    ErrMaps[3004] = "db: set task status as cancelled has failed"
    ErrMaps[3005] = "db: set task status as hang-up has failed"
    ErrMaps[3006] = "db: set task status as failed has failed"
    ErrMaps[3007] = "db: set task status as waited has failed"
    ErrMaps[3008] = "db: find upload has failed"

    // Ws-server error message.
    ErrMaps[5001] = "emit resume has failed"
    ErrMaps[5002] = "emit pause has failed"
    ErrMaps[5003] = "emit abort has failed"
    ErrMaps[5004] = "emit watch has failed"
    ErrMaps[5100] = "act-%s is calling: %s"
    ErrMaps[5101] = "request data is illegal in act-%s: %s"
    ErrMaps[5102] = "act-%s: response errors occurred"
    ErrMaps[5103] = "act-%s: no job are processing"
}