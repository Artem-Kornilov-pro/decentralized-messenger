"""Merkle tree for efficient chat history integrity proofs."""

from __future__ import annotations

import hashlib


def _hash(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def _combine(left: str, right: str) -> str:
    return _hash((left + right).encode())


def build_merkle_root(leaf_hashes: list[str]) -> str:
    """Compute the Merkle root from a list of leaf hashes.

    Returns an empty-string sentinel when there are no leaves.
    Duplicates the last leaf when the count is odd (standard padding).
    """
    if not leaf_hashes:
        return ""

    layer = list(leaf_hashes)
    while len(layer) > 1:
        if len(layer) % 2 == 1:
            layer.append(layer[-1])
        layer = [_combine(layer[i], layer[i + 1]) for i in range(0, len(layer), 2)]

    return layer[0]
