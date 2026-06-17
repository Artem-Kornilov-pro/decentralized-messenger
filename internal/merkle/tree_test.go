package merkle

import "testing"

func TestRootEmpty(t *testing.T) {
	if Root(nil) != "" {
		t.Fatal("empty leaves should give empty root")
	}
}

func TestRootSingleLeaf(t *testing.T) {
	if Root([]string{"abc"}) != "abc" {
		t.Fatal("single leaf root should be the leaf itself")
	}
}

func TestRootDeterministic(t *testing.T) {
	leaves := []string{"a", "b", "c", "d"}
	if Root(leaves) != Root(leaves) {
		t.Fatal("root not deterministic")
	}
}

func TestRootOrderSensitive(t *testing.T) {
	if Root([]string{"a", "b"}) == Root([]string{"b", "a"}) {
		t.Fatal("root should depend on leaf order")
	}
}

func TestProofRoundTrip(t *testing.T) {
	leaves := []string{"a", "b", "c", "d", "e"}
	for i, leaf := range leaves {
		proof, root := Proof(leaves, i)
		if root != Root(leaves) {
			t.Fatalf("proof root mismatch at index %d", i)
		}
		if !VerifyProof(leaf, proof, root) {
			t.Fatalf("proof failed to verify at index %d", i)
		}
		if VerifyProof("wrong", proof, root) {
			t.Fatalf("proof verified a wrong leaf at index %d", i)
		}
	}
}

func TestProofOutOfRange(t *testing.T) {
	if proof, root := Proof([]string{"a"}, 5); proof != nil || root != "" {
		t.Fatal("out-of-range index should return nil proof")
	}
}
