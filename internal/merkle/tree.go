// Package merkle builds Merkle roots and inclusion proofs over chat log entry
// hashes, enabling any participant to prove integrity of the history without
// holding the full chain.
package merkle

import (
	"crypto/sha256"
	"encoding/hex"
)

func hashPair(left, right string) string {
	sum := sha256.Sum256([]byte(left + right))
	return hex.EncodeToString(sum[:])
}

// Root computes the Merkle root over the ordered leaf hashes. It returns the
// empty string for no leaves and the single leaf itself for one leaf. When a
// layer has an odd number of nodes the last node is duplicated.
func Root(leaves []string) string {
	if len(leaves) == 0 {
		return ""
	}
	layer := make([]string, len(leaves))
	copy(layer, leaves)

	for len(layer) > 1 {
		if len(layer)%2 == 1 {
			layer = append(layer, layer[len(layer)-1])
		}
		next := make([]string, 0, len(layer)/2)
		for i := 0; i < len(layer); i += 2 {
			next = append(next, hashPair(layer[i], layer[i+1]))
		}
		layer = next
	}
	return layer[0]
}

// ProofNode is one step of an inclusion proof: a sibling hash and which side it
// sits on relative to the running hash.
type ProofNode struct {
	Hash   string `json:"hash"`
	IsLeft bool   `json:"is_left"`
}

// Proof returns the inclusion proof for the leaf at the given index, alongside
// the resulting Merkle root. It returns (nil, "") if the index is out of range.
func Proof(leaves []string, index int) ([]ProofNode, string) {
	if index < 0 || index >= len(leaves) {
		return nil, ""
	}
	layer := make([]string, len(leaves))
	copy(layer, leaves)

	var proof []ProofNode
	idx := index
	for len(layer) > 1 {
		if len(layer)%2 == 1 {
			layer = append(layer, layer[len(layer)-1])
		}
		if idx%2 == 0 {
			proof = append(proof, ProofNode{Hash: layer[idx+1], IsLeft: false})
		} else {
			proof = append(proof, ProofNode{Hash: layer[idx-1], IsLeft: true})
		}
		next := make([]string, 0, len(layer)/2)
		for i := 0; i < len(layer); i += 2 {
			next = append(next, hashPair(layer[i], layer[i+1]))
		}
		layer = next
		idx /= 2
	}
	return proof, layer[0]
}

// VerifyProof recomputes the root from a leaf and its inclusion proof and
// reports whether it matches the expected root.
func VerifyProof(leaf string, proof []ProofNode, root string) bool {
	running := leaf
	for _, node := range proof {
		if node.IsLeft {
			running = hashPair(node.Hash, running)
		} else {
			running = hashPair(running, node.Hash)
		}
	}
	return running == root
}
