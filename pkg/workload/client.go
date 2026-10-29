package workload

import (
	"github.com/ayush00git/quorum/pkg/raft"
	"github.com/ayush00git/quorum/pkg/sim"
)

// client is one Jepsen-style client slot: at most one outstanding op. When
// an op times out its outcome is unknown, so the process retires and the
// slot continues as a fresh process id.
type client struct {
	env  *sim.Env
	rec  *Recorder
	rand *sim.Rand

	process int
	seq     int
	leader  string

	opID    int // -1 when idle
	cmd     raft.Command
	attempt int
	node    string
	dup     bool // a request of this op was duplicated by the network
	timeout sim.TimerID
	think   sim.TimerID
	retry   sim.TimerID
}

func (c *client) retire() {
	c.process = c.rec.newProcess()
	c.seq = 0
	c.opID = -1
}

func (c *client) OnStart() { c.scheduleNext() }

func (c *client) scheduleNext() {
	c.think = c.env.SetTimer(c.rand.Range(0, c.rec.cfg.ThinkMaxMs))
}

func (c *client) OnTimer(id sim.TimerID) {
	switch id {
	case c.think:
		c.invoke()
	case c.timeout:
		if c.opID >= 0 {
			c.rec.complete(c.opID, StatusInfo, c.node, 0, nil)
			c.leader = ""
			c.retire()
			c.scheduleNext()
		}
	case c.retry:
		if c.opID >= 0 {
			c.sendAttempt(c.pickNode(""))
		}
	}
}

func (c *client) invoke() {
	now := c.env.Now()
	if now >= c.rec.stopAt {
		return
	}
	cfg := c.rec.cfg
	key := cfg.Keys[c.rand.Intn(len(cfg.Keys))]
	c.seq++
	c.cmd = raft.Command{Kind: raft.CmdRead, Key: key, Client: c.process, Seq: c.seq}
	op := Op{Process: c.process, Kind: KindRead, Key: key, InvokeAt: now}
	if c.rand.Float64() >= cfg.ReadRatio {
		v := c.rec.uniqueValue(key)
		c.cmd.Kind, c.cmd.Value = raft.CmdWrite, v
		op.Kind, op.Value = KindWrite, &v
	}
	c.opID = c.rec.invoke(op)
	c.attempt = 0
	c.dup = false
	c.timeout = c.env.SetTimer(cfg.OpTimeoutMs)
	target := c.leader
	if c.cmd.Kind == raft.CmdRead && cfg.ReadFrom == ReadFromAny {
		target = cfg.Nodes[c.rand.Intn(len(cfg.Nodes))]
	}
	c.sendAttempt(c.pickNode(target))
}

// pickNode returns preferred if set, otherwise a random node other than the
// one that just refused.
func (c *client) pickNode(preferred string) string {
	if preferred != "" {
		return preferred
	}
	nodes := c.rec.cfg.Nodes
	for {
		n := nodes[c.rand.Intn(len(nodes))]
		if n != c.node || len(nodes) == 1 {
			return n
		}
	}
}

func (c *client) sendAttempt(node string) {
	c.attempt++
	c.node = node
	if c.env.Send(node, raft.Request{Attempt: c.attempt, Op: c.cmd}) {
		c.dup = true
	}
}

func (c *client) OnMessage(from string, msg any) {
	r, ok := msg.(raft.Reply)
	if !ok || c.opID < 0 || r.Client != c.process || r.Seq != c.seq {
		return
	}
	if r.OK {
		c.finish(StatusOK, from, r.Result)
		if !(c.cmd.Kind == raft.CmdRead && c.rec.cfg.ReadFrom == ReadFromAny) {
			c.leader = from
		}
		return
	}
	if !r.NotLeader || r.Attempt != c.attempt {
		return
	}
	c.leader = r.Leader
	if c.attempt >= c.rec.cfg.MaxAttempts {
		// Every attempt was refused before anything was appended, so the op
		// definitely did not happen, unless a duplicate of a request is
		// still in flight and may yet reach a leader.
		status := StatusFail
		if c.dup {
			status = StatusInfo
		}
		c.finish(status, from, nil)
		return
	}
	if r.Leader != "" && r.Leader != from {
		c.sendAttempt(r.Leader)
		return
	}
	c.retry = c.env.SetTimer(c.rec.cfg.RetryMs)
}

func (c *client) finish(status, node string, result *int) {
	c.rec.complete(c.opID, status, node, c.env.Now(), result)
	if status == StatusInfo {
		c.retire()
	} else {
		c.opID = -1
	}
	c.timeout, c.retry = 0, 0
	c.scheduleNext()
}
