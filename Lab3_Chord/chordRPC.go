package main

import (
	"fmt"
	"log"
	"math/big"
)

//////////////////////////////////////////////
/////////////// RPC calls ////////////////////
//////////////////////////////////////////////

func (chord *Chord) CallNotify(nodeAddress NodeAddress) {
	notifyReq := NotifyRequest{chord.node.NodeAddress, chord.node.Identifier}
	resp := NotifyResponse{}
	if nodeAddress == chord.node.NodeAddress {
		chord.tracer.Trace("Trying to notify myself...")
	}
	if !chord.call("Chord.Notify", &notifyReq, &resp, string(nodeAddress)) {
		// Cannot notify successor => find new predecessor from FIND
		chord.tracer.Trace("Node to notify died")
		return
	}

	if resp.Success {
		chord.node.Successors[0] = nodeAddress
		return
	}
}

func (chord *Chord) CallJoin(nodeAddress NodeAddress) {
	joinReq := JoinRequest{chord.node.NodeAddress, chord.node.Identifier}
	Response := Response{}
	if !chord.call("Chord.Join", &joinReq, &Response, string(nodeAddress)) {
		log.Fatal("Cannot join specified node on chord ring")
	}
	// Join successful
	chord.tracer.Trace("### Received closest predecessor ###\nIsSucc: %v, Received: %s\nId: %01x\n",
		Response.IsSuccessor, Response.NodeAddress, &Response.Identifier)

	if Response.IsSuccessor {
		// TODO: notify node that I am new predecessor Notify node
		//Shift successors
		defer chord.CallNotify(Response.NodeAddress)
	} else {
		chord.tracer.Trace("Retrying join with %s", Response.NodeAddress)
		go chord.CallJoin(Response.NodeAddress)
	}
}

func (chord *Chord) CallFindSuccessor(findReq *FindRequest, response *Response) bool {
	if findReq.QueryNodeAddress == chord.node.NodeAddress {
		chord.tracer.Trace("Find succ on my self...")
		response.NodeAddress = "X"
		response.IsSuccessor = true
	}
	if !chord.call("Chord.FindSuccessor", &findReq, response, string(findReq.QueryNodeAddress)) {
		chord.tracer.Trace("Successor not reachable, contact other users")
		return false
	}
	return true
}

func (chord *Chord) CallStabilize() {

	stabilizeReq := StabilizeRequest{chord.node.NodeAddress, chord.node.Identifier}
	succ := chord.node.Successors[0]
	if succ == "X" { // S
		chord.findNewSucc()
	} else if succ == chord.node.NodeAddress {
		// No need to stabilize when we are alone
		return
	}
	stabilizeResp := StabilizeResponse{}
	if !chord.call("Chord.Stabilize", &stabilizeReq, &stabilizeResp, string(succ)) {
		// Our successor died: use finger table to find closest new node
		chord.clearDeadSucc()
		return
	}

	if !stabilizeResp.YouGood {
		// Notify new pred !
		chord.tracer.Trace("Stabilize: Currsucc: %s, SuccsPred %s", succ, stabilizeResp.NewPredAddress)
		if stabilizeResp.NewPredAddress != chord.node.NodeAddress {
			defer chord.CallNotify(stabilizeResp.NewPredAddress)
		} else {
			chord.tracer.Trace("Incoherent state: THIS SHOULD NOT HAPPEN")
			return
		}
		return

	}
	chord.verifySuccs(&stabilizeResp)
}

func (chord *Chord) CallCheckPred() bool {
	if chord.node.Predecessor == "X" {
		return false
	}
	deadCheck := DeadCheck{}
	return chord.call("Chord.CheckPred", &deadCheck, &deadCheck, string(chord.node.Predecessor))
}

func (chord *Chord) CallDuplication(storeFileReq *StoreFileRequest) {
	// Request duplication to your successor
	storeFileResp := StoreFileResponse{}
	if chord.node.Successors[0] == "X" {
		chord.tracer.Trace("Duplication aborted: successor died")
		return
	}

	chord.tracer.Trace("Duplicating file: %s to our successor %s", storeFileReq.FileName, chord.node.Successors[0])
	if !chord.call("Chord.StoreFile", storeFileReq, &storeFileResp, string(chord.node.Successors[0])) {
		chord.tracer.Trace("Unable to contact successor")
		return
	}

	if storeFileResp.FileStatus != FILEOK {
		if storeFileResp.FileStatus == CREATEERR {
			chord.tracer.Trace("Store file failed: %s: CANNOT CREATE FILE", storeFileReq.FileName)
		} else if storeFileResp.FileStatus == WRITEERR {
			chord.tracer.Trace("Store file failed: %s: CANNOT WRITE TO FILE", storeFileReq.FileName)
		}
	}
}

func (chord *Chord) CallStoreFile(storeFileReq *StoreFileRequest) {
	// Use find to know what file to look for
	storeFileResp := StoreFileResponse{}
	destinationNode := chord.resolveIdentifier(&storeFileReq.FileIdentifier)

	if !chord.call("Chord.StoreFile", storeFileReq, &storeFileResp, string(destinationNode)) {
		chord.tracer.Trace("Unable to store file at destination")
		return
	}

	if storeFileResp.FileStatus != FILEOK {
		if storeFileResp.FileStatus == CREATEERR {
			chord.tracer.Trace("Store file failed: %s: CANNOT CREATE FILE", storeFileReq.FileName)
		} else if storeFileResp.FileStatus == WRITEERR {
			chord.tracer.Trace("Store file failed: %s: CANNOT WRITE TO FILE", storeFileReq.FileName)
		}
	}
}

func (chord *Chord) CallLookup(retreiveFileReq *RetreiveFileRequest) {
	retreiveFileResp := RetreiveFileResponse{}
	destinationNode := chord.resolveIdentifier(&retreiveFileReq.FileIdentifier)
	fmt.Printf("Node: %s\n", destinationNode)
	if !chord.call("Chord.LookUp", retreiveFileReq, &retreiveFileResp, string(destinationNode)) {
		// Una
		chord.tracer.Trace("Unable to store file at destination")
		return
	}

	if retreiveFileResp.FileStatus != FILEOK {
		if retreiveFileResp.FileStatus == NOSUCHFILE {
			chord.tracer.Trace("File look up fail: %s: NO SUCH FILE", retreiveFileReq.FileName)
		} else if retreiveFileResp.FileStatus == READERR {
			chord.tracer.Trace("File look up fail: %s: CANNOT READ FILE", retreiveFileReq.FileName)
		}
		return
	}

	fmt.Printf("\n## Lookup ##\nNode identifier: %x\nNode address: %s\n####### Content ##########\n%s\n\n", &retreiveFileResp.Identifier, retreiveFileResp.NodeAddress, retreiveFileResp.FileContent)

}

func (chord *Chord) resolveIdentifier(identifier *big.Int) NodeAddress {
	// Look in finger table
	response := Response{}
	retCode := chord.find(identifier, &response)
	chord.tracer.Trace("File %x found at %x", identifier, &chord.node.Identifier)
	if retCode != PASSALONG {
		// Return what was found in find
		return response.NodeAddress
	}
	if response.NodeAddress == chord.node.NodeAddress {
		return chord.node.NodeAddress
	}

	// Ask node from finger table
	findReq := FindRequest{*identifier, response.NodeAddress}
	for resolveIter := 0; resolveIter < 32; resolveIter++ {
		if findReq.QueryNodeAddress == chord.node.NodeAddress {
			return chord.node.NodeAddress
		}
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
