package common

// PutArgs holds arguments for the Put RPC.
type PutArgs struct {
	Key       string
	Value     string
	ForwardTo string // Address of the next worker to replicate to (for Chain Replication)
}

// PutReply holds the reply for the Put RPC.
type PutReply struct {
	// No return value needed for Put, but we could add an error code here if we wanted.
}

// GetArgs holds arguments for the Get RPC.
type GetArgs struct {
	Key string
}

// GetReply holds the reply for the Get RPC.
type GetReply struct {
	Value string
	Found bool
}
