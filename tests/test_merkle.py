"""Tests for Merkle tree construction."""

from decentralized_messenger.merkle.tree import build_merkle_root


def test_empty_returns_empty_string() -> None:
    assert build_merkle_root([]) == ""


def test_single_leaf_is_itself() -> None:
    root = build_merkle_root(["abc"])
    assert root == "abc"


def test_deterministic() -> None:
    leaves = ["a", "b", "c", "d"]
    assert build_merkle_root(leaves) == build_merkle_root(leaves)


def test_different_order_gives_different_root() -> None:
    assert build_merkle_root(["a", "b"]) != build_merkle_root(["b", "a"])


def test_odd_number_of_leaves() -> None:
    root = build_merkle_root(["a", "b", "c"])
    assert isinstance(root, str) and len(root) == 64
