package main

import (
	"crypto/sha1"
	"fmt"
	"log"
	"math/big"
)

// Usful utility functions for chord

func hashString(elt string) *big.Int {
	hasher := sha1.New()
	hasher.Write([]byte(elt))
	return new(big.Int).SetBytes(hasher.Sum(nil))
}

const keySize = sha1.Size * 8

var two = big.NewInt(2)
var hashMod = new(big.Int).Exp(big.NewInt(2), big.NewInt(keySize), nil)

func jump(address string, fingerentry int) *big.Int {
	n := hashString(address)
	fingerentryminus1 := big.NewInt(int64(fingerentry) - 1)
	jump := new(big.Int).Exp(two, fingerentryminus1, nil)
	sum := new(big.Int).Add(n, jump)

	return new(big.Int).Mod(sum, hashMod)
}

func getIdentifier(address string, specified string) *big.Int {
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
		return n
	}
}

func verifyRange(stabilize, fixFingers, checkPred int) bool {
	stab := stabilize > 0 && stabilize < 60001
	finger := fixFingers > 0 && fixFingers < 60001
	check := checkPred > 0 && checkPred < 60001
	return stab && finger && check
}

func printHash(hash *big.Int) {
	fmt.Printf("Hash: %040x\n", hash)
}
