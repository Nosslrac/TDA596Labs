package main

import (
	"crypto/sha1"
	"fmt"
	"log"
	"math/big"
)

// Usful utility functions for chord

func hashString(elt NodeAddress) *big.Int {
	hasher := sha1.New()
	hasher.Write([]byte(elt))
	return mod(new(big.Int).SetBytes(hasher.Sum(nil)))
}

const keySize = 4

var two = big.NewInt(2)
var hashMod = new(big.Int).Exp(two, big.NewInt(keySize), nil)

func jump(address NodeAddress, fingerentry int) *big.Int {
	n := hashString(address)
	fingerentryminus1 := big.NewInt(int64(fingerentry) - 1)
	jump := new(big.Int).Exp(two, fingerentryminus1, nil)
	sum := new(big.Int).Add(n, jump)

	return new(big.Int).Mod(sum, hashMod)
}

func jumpIdentifier(identifier *big.Int, fingerentry int) *big.Int {
	fingerentryminus1 := big.NewInt(int64(fingerentry) - 1)
	jump := new(big.Int).Exp(two, fingerentryminus1, nil)
	sum := new(big.Int).Add(identifier, jump)

	return new(big.Int).Mod(sum, hashMod)
}

func mod(input *big.Int) *big.Int {
	return new(big.Int).Mod(input, hashMod)
}

func between(start, elt, end *big.Int, inclusive bool) bool {
	if end.Cmp(start) > 0 {
		return (start.Cmp(elt) < 0 && elt.Cmp(end) < 0) || (inclusive && elt.Cmp(end) == 0)
	} else {
		return start.Cmp(elt) < 0 || elt.Cmp(end) < 0 || (inclusive && elt.Cmp(end) == 0)
	}
}

func getIdentifier(address NodeAddress, specified string) *big.Int {
	if specified == "XXX" {
		//Not specified -> generate based on address
		return hashString(address)
	} else {
		fmt.Printf("Hello: %s\n", specified)
		n := new(big.Int)
		n, ok := n.SetString(specified, 16)
		if !ok {
			log.Fatalf("Specified identifier parse error: %s\n", specified)
		}
		return n.Mod(n, hashMod)
	}
}

func verifyRange(stabilize, fixFingers, checkPred int) bool {
	stab := stabilize > 0 && stabilize < 60001
	finger := fixFingers > 0 && fixFingers < 60001
	check := checkPred > 0 && checkPred < 60001
	return stab && finger && check
}

func printHash(hash *big.Int) {
	x := new(big.Int).Mod(hash, hashMod)
	fmt.Printf("Hash: %01x\n", x)
}
