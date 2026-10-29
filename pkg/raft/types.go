// Package raft implements a Raft replicated key-value store whose behavior
// can be altered by planted bug flags.
package raft

// Command kinds.
const (
	CmdNoop  = "noop"
	CmdRead  = "read"
	CmdWrite = "write"
	CmdCas   = "cas"
)

// Command is a state machine command. Client and Seq identify the client
// session for deduplication of retried requests.
type Command struct {
	Kind     string
	Key      string
	Value    int
	Expected int
	Client   int
	Seq      int
}

// Entry is one log entry.
type Entry struct {
	Term int
	Cmd  Command
}

// RequestVote asks for a vote.
type RequestVote struct {
	Term      int
	Candidate string
	LastIndex int
	LastTerm  int
}

// VoteReply answers RequestVote.
type VoteReply struct {
	Term    int
	Granted bool
}

// AppendEntries replicates entries and doubles as a heartbeat.
type AppendEntries struct {
	Term      int
	Leader    string
	PrevIndex int
	PrevTerm  int
	Entries   []Entry
	Commit    int
	Round     int // lease heartbeat round, echoed in the reply
}

// AppendReply answers AppendEntries. Match is the last index known to match
// on success; Conflict is where the leader should retry on failure.
type AppendReply struct {
	Term     int
	Success  bool
	Match    int
	Conflict int
	Round    int
}

// Request is a client request. Attempt is echoed back so a client can tell
// replies to different attempts of the same operation apart.
type Request struct {
	Attempt int
	Op      Command
}

// Reply answers a Request. Result is the value read (nil when the key is
// unset); CasOK reports whether a cas took effect.
type Reply struct {
	Client    int
	Seq       int
	Attempt   int
	OK        bool
	NotLeader bool
	Leader    string
	Result    *int
	CasOK     bool
}
