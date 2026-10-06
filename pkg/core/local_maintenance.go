package core

import "sync"

// Native snapshots briefly stop request and scheduler mutations, including file
// attachments. Separate gates allow a request to synchronously run a cron job.
var LocalRequestGate sync.RWMutex
var LocalCronGate sync.RWMutex
