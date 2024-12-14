package main

import (
	"fmt"
	"log"
	"math/big"
)

//////////////////////////////////////////////
/////////////// RPC calls ////////////////////
//////////////////////////////////////////////

// Recursively notifies until it gets a successful notify
func (chord *Chord) CallNotify(nodeAddress NodeAddress) {
	notifyReq := NotifyRequest{chord.node.NodeAddress, chord.node.Identifier}
	resp := NotifyResponse{}
	if !call("Chord.Notify", &notifyReq, &resp, string(nodeAddress)) {
		// Cannot notify successor => find new predecessor from FIND
		log.Fatal("Cannot notify successor")
	}

	if resp.Success {
		chord.insertSuccessor(nodeAddress)
		return
	}
	chord.tracer.Trace("This is very weird, should not happen")
}

func (chord *Chord) CallJoin(nodeAddress NodeAddress) {
	joinReq := JoinRequest{chord.node.NodeAddress, chord.node.Identifier}
	Response := Response{}
	fmt.Print(nodeAddress)
	if !call("Chord.Join", &joinReq, &Response, string(nodeAddress)) {
		log.Fatal("Cannot join specified node on chord ring")
	}
	// Join successful
	chord.tracer.Trace("### Received closest predecessor ###\nIsSucc: %v, Received: %s\nId: %01x\n",
		Response.IsSuccessor, Response.NodeAddress, &Response.Identifier)

	if Response.IsSuccessor {
		// TODO: notify node that I am new predecessor Notify node
		//Shift successors
		chord.CallNotify(Response.NodeAddress)
	} else {
		// Contact new node to get successor
		chord.CallJoin(Response.NodeAddress)
	}
}

func (chord *Chord) CallFindSuccessor(findReq *FindRequest, reponse *Response) bool {
	if !call("Chord.FindSuccessor", &findReq, reponse, string(findReq.QueryNodeAddress)) {
		chord.tracer.Trace("Successor not reachable, contact other users")
		return false
	}
	return true
}

func (chord *Chord) CallStabilize() {
	chord.chordSync.Lock()
	stabilizeReq := StabilizeRequest{chord.node.NodeAddress, chord.node.Identifier}
	succ := chord.node.Successors[0]
	chord.chordSync.Unlock()
	stabilizeResp := StabilizeResponse{}
	if !call("Chord.Stabilize", &stabilizeReq, &stabilizeResp, string(succ)) {
		// Our successor died: use finger table to find closest new node
		chord.tracer.Trace("Our succ has died, try to find new succ")
		chord.chordSync.Lock()
		chord.clearDeadNode(succ, true)
		chord.chordSync.Unlock()
		return
	}

	if !stabilizeResp.YouGood {
		// Notify new pred !
		chord.tracer.Trace("Stabilize: Currsucc: %s, SuccsPred %s", succ, stabilizeResp.NewPredAddress)
		if stabilizeResp.NewPredAddress == chord.node.NodeAddress {
			log.Fatal("What is wrooong")
		}
		chord.CallNotify(stabilizeResp.NewPredAddress)
	}
}

func (chord *Chord) CallCheckPred() bool {
	deadCheck := DeadCheck{}
	return call("Chord.CheckPred", &deadCheck, &deadCheck, string(chord.node.Predecessor))
}

func (chord *Chord) CallStoreFile(storeFileReq *StoreFileRequest) {
	// Use find to know what file to look for
	storeFileResp := StoreFileResponse{}
	destinationNode := chord.resolveIdentifier(&storeFileReq.FileIdentifier)
	chord.tracer.Trace("Store file at id: %x => resolved to %s", storeFileReq.FileIdentifier, destinationNode)

	if !call("Chord.StoreFile", &storeFileReq, &storeFileResp, string(destinationNode)) {
		// Una
		chord.tracer.Trace("Unable to store file at destination")
		return
	}
}

func (chord *Chord) resolveIdentifier(identifier *big.Int) NodeAddress {
	// Look in finger table
	response := Response{}
	retCode := chord.find(identifier, &response)

	if retCode != PASSALONG {
		// Return what was found in find
		return response.NodeAddress
	}
	// Ask node from finger table
	findReq := FindRequest{response.Identifier, response.NodeAddress}
	for resolveIter := 0; resolveIter < 32; resolveIter++ {
		resp := Response{}
		if !chord.CallFindSuccessor(&findReq, &resp) {
			chord.tracer.Trace("Failure to contact node which should store the file %s: Abort stor", response.NodeAddress)
			return "X"
		}
		if resp.IsSuccessor {
			return resp.NodeAddress
		}
		// Query a better node for a better result
		findReq.QueryNodeAddress = resp.NodeAddress
	}
	return "X"
}
