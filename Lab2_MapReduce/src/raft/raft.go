package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	//	"bytes"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	//	"6.5840/labgob"
	"6.5840/labrpc"
)

type ServerState uint
type Term int
type ServerId int

// Timings
const ELECTION_TIMEOUT int64 = 400
const HEARTBEAT_INTERVAL int64 = 100

const FOLLOWER ServerState = 1
const CANDIDATE ServerState = 2
const LEADER ServerState = 3

type LogEntry struct {
	// Add log info here
	Command  int
	LogTerm  Term
	LogIndex int
}

// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in part 2D you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh, but set CommandValid to false for these
// other uses.
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int

	// For 2D:
	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}

// A Go object implementing a single Raft peer.
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	//Election state
	hasReceivedHeartbeat bool

	serverState ServerState

	currentTerm   Term
	numPeers      int
	receivedVotes int
	votedFor      ServerId
	log           []LogEntry
	applyChan     *chan ApplyMsg

	// Volatile server state
	commitIndex int
	lastApplied int

	// Only leader volatile state (size = num peers)
	nextIndex  []int
	matchIndex []int
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool
	// Your code here (2A).
	rf.mu.Lock()
	term = int(rf.currentTerm)
	isleader = rf.serverState == LEADER
	rf.mu.Unlock()

	return term, isleader
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
// before you've implemented snapshots, you should pass nil as the
// second argument to persister.Save().
// after you've implemented snapshots, pass the current snapshot
// (or nil if there's not yet a snapshot).
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// raftstate := w.Bytes()
	// rf.persister.Save(raftstate, nil)
}

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (2D).

}

// example RequestVote RPC arguments structure.
// field names must start with capital letters!
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	CandidateTerm Term
	CandidateId   ServerId
	LastLogIndex  int
	LastLogTerm   int
}

// example RequestVote RPC reply structure.
// field names must start with capital letters!
type RequestVoteReply struct {
	// Your data here (2A).
	ResponseTerm Term
	VoteGrandted bool
}

type AppendEntriesArgs struct {
	LeaderTerm Term
	LeaderId   ServerId

	PrevLogIndex int
	PrevLogTerm  Term
	Entries      []LogEntry

	LeaderCommitIndex int
}

type AppendEntriesReply struct {
	FollowersTerm Term
	Success       bool
}

// example RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if args.CandidateTerm < rf.currentTerm {
		reply.ResponseTerm = rf.currentTerm
		reply.VoteGrandted = false
		return
	}

	if args.CandidateTerm > rf.currentTerm {
		// New term => new vote + set ourself as FOLLOWER
		rf.votedFor = -1
		rf.receivedVotes = 0
		rf.currentTerm = args.CandidateTerm
		if rf.serverState == LEADER {
			reason := fmt.Sprintf("Candidate %d: has higher term = %d than me (%d) = %d\n", args.CandidateId, args.CandidateTerm, rf.me, rf.currentTerm)
			rf.stepDown(reason)
		}
	}

	logUpToDate := args.LastLogIndex >= rf.nextIndex[args.CandidateId]-1

	if (rf.votedFor == -1 || rf.votedFor == args.CandidateId) && logUpToDate {
		rf.votedFor = args.CandidateId
		rf.currentTerm = args.CandidateTerm
		rf.serverState = FOLLOWER
		reply.VoteGrandted = true
		DPrintf("VOTE GRANTED: candidate %d: requested our (%d) vote for term %d\n", args.CandidateId, rf.me, args.CandidateTerm)
		return
	}
	reply.ResponseTerm = rf.currentTerm
	reply.VoteGrandted = false
}

// Append entries / heartbeat
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if args.LeaderTerm < rf.currentTerm {
		DPrintf("APPEND: My (%d) term = %d is higher than your (%d) term = %d \n", rf.me, rf.currentTerm, args.LeaderId, args.LeaderTerm)
		reply.FollowersTerm = rf.currentTerm
		reply.Success = false
		return
	}

	if args.LeaderTerm > rf.currentTerm {
		// We didn't parttake in election
		rf.currentTerm = args.LeaderTerm
		if rf.serverState == LEADER {
			reason := fmt.Sprintf("Leader(%d) receive heartbeat with higher term from %d\n", rf.me, args.LeaderId)
			rf.stepDown(reason)
		} else {
			DPrintf("APPEND: My (%d) term isn't matching the leaders, maybe we didn't partake in the election\n", rf.me)
		}
	}
	reply.Success = true

	// TODO: Resolving logs, not sure how they should be handled

	// Leader contacted us
	rf.hasReceivedHeartbeat = true
}

// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

func (rf *Raft) sendHeartBeat(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	isLeader := rf.serverState == LEADER
	if !isLeader {
		return -1, -1, false
	}
	// Your code here (2B).

	return rf.commitIndex + 1, int(rf.currentTerm), isLeader
}

// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

// Executed under lock
func (rf *Raft) stepDown(reason string) {
	DPrintf("STEP DOWN: %s\n", reason)
	rf.serverState = FOLLOWER
}

func (rf *Raft) leaderSendHeartBeats() {
	for !rf.killed() {
		time.Sleep(time.Duration(HEARTBEAT_INTERVAL) * time.Millisecond)
		rf.mu.Lock()
		if rf.serverState == LEADER {
			rf.sendHeartBeatToAllPeers()
		}
		rf.mu.Unlock()
	}
	DPrintf("Exiting: leaderSendHeartbeat")
}

func (rf *Raft) checkElectionTimeout() {
	for !rf.killed() {
		rf.mu.Lock()
		rf.hasReceivedHeartbeat = false
		currTerm := rf.currentTerm
		rf.mu.Unlock()

		// pause for a random amount of time between 300 and 600
		// milliseconds.
		ms := ELECTION_TIMEOUT + (rand.Int63() % 300)
		time.Sleep(time.Duration(ms) * time.Millisecond)

		// Your code here (2A)
		// Check if a leader election should be started.
		rf.mu.Lock()
		if rf.serverState == LEADER {
			rf.mu.Unlock()
			continue
		}

		sameTerm := currTerm == rf.currentTerm
		// Check if should start election
		if !rf.hasReceivedHeartbeat && sameTerm {
			rf.receivedVotes = 1 // Vot from ourself
			rf.currentTerm++
			rf.votedFor = ServerId(rf.me)
			rf.serverState = CANDIDATE
			DPrintf("I (%d) haven't received heartbeats, running as CANDIDATE for term %d\n", rf.me, rf.currentTerm)
			//TODO: ask other peers for vote
			rf.askAllPeersForVote()
		}
		rf.mu.Unlock()
	}
	DPrintf("Exiting: checkElection")
}

func (rf *Raft) askForVote(peer int) {
	rf.mu.Lock()
	reqVote := RequestVoteArgs{rf.currentTerm, ServerId(rf.me), int(rf.commitIndex), int(rf.lastApplied)}
	rf.mu.Unlock()
	replyVote := RequestVoteReply{}
	backOffFactor := 1.5
	baseTime := 1000.0
	for ok := rf.sendRequestVote(peer, &reqVote, &replyVote); !ok; {
		if rf.killed() {
			return
		}
		//RPC failed retry after some time
		// DPrintf("RPC failed: retrying in 1 s\n")
		time.Sleep(time.Millisecond * time.Duration(baseTime))
		baseTime *= backOffFactor
	}
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if replyVote.VoteGrandted {
		DPrintf("ASK VOTE GRANTED: I (%d) got vote from %d for term %d\n", rf.me, peer, rf.currentTerm)
		rf.receivedVotes++
		if rf.receivedVotes > rf.numPeers/2 && rf.serverState != LEADER {
			DPrintf("STEP UP: I (%d) received %d votes in term %d\n", rf.me, rf.receivedVotes, rf.currentTerm)
			rf.serverState = LEADER
		}
	}
}

// Executed under lock
func (rf *Raft) askAllPeersForVote() {
	for peer := range rf.peers {
		if peer != rf.me {
			go rf.askForVote(peer)
		}
	}
}

func (rf *Raft) sendHeartBeatToPeer(peer int) {
	rf.mu.Lock()
	// TODO: arguments are not correct
	reqAppend := AppendEntriesArgs{rf.currentTerm, ServerId(rf.me), int(rf.commitIndex), rf.log[rf.commitIndex].LogTerm, nil, rf.commitIndex}
	rf.mu.Unlock()
	replyAppend := AppendEntriesReply{}
	backOffFactor := 1.5
	baseTime := 1000.0
	for ok := rf.sendHeartBeat(peer, &reqAppend, &replyAppend); !ok; {
		rf.mu.Lock()
		if rf.killed() || rf.serverState != LEADER {
			rf.mu.Unlock()
			return
		}
		rf.mu.Unlock()

		//RPC failed retry after some time
		DPrintf("HEARTBEAT: My (%d) heartbeat to %d failed\n", rf.me, peer)
		time.Sleep(time.Millisecond * time.Duration(baseTime))
		baseTime *= backOffFactor
	}
	rf.mu.Lock()
	defer rf.mu.Unlock()
	// TODO: something with response
	if !replyAppend.Success {
		// They have a larger term => we should step down
		rf.currentTerm = replyAppend.FollowersTerm
		if rf.serverState == LEADER {
			reason := fmt.Sprintf("I (%d) with term %d have lower term than %d = %d\n", rf.me, rf.currentTerm, replyAppend.FollowersTerm, peer)
			rf.stepDown(reason)
		}
	}

}

// Executed under lock
func (rf *Raft) sendHeartBeatToAllPeers() {
	for peer := range rf.peers {
		if peer != rf.me {
			go rf.sendHeartBeatToPeer(peer)
		}
	}
}

// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).
	rf.votedFor = -1 // -1 used as nil vote
	rf.currentTerm = 0
	rf.hasReceivedHeartbeat = false
	rf.serverState = FOLLOWER
	rf.applyChan = &applyCh

	rf.numPeers = len(peers)

	rf.nextIndex = make([]int, rf.numPeers)
	rf.matchIndex = make([]int, rf.numPeers)

	rf.log = append(rf.log, LogEntry{})

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	DPrintf("Starting raft with %d peers.\n", rf.numPeers)

	// start ticker goroutine to start elections
	// go rf.ticker()
	go rf.checkElectionTimeout()
	go rf.leaderSendHeartBeats()

	return rf
}
