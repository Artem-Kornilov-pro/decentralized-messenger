"""Tests for Ed25519 key operations."""

from decentralized_messenger.crypto.keys import generate_keypair, sign, verify


def test_sign_and_verify() -> None:
    private_key, public_key = generate_keypair()
    data = b"hello, messenger"
    signature = sign(private_key, data)
    assert verify(public_key, data, signature)


def test_verify_rejects_tampered_data() -> None:
    private_key, public_key = generate_keypair()
    data = b"original message"
    signature = sign(private_key, data)
    assert not verify(public_key, b"tampered message", signature)


def test_verify_rejects_wrong_key() -> None:
    private_key, _ = generate_keypair()
    _, other_public = generate_keypair()
    data = b"some data"
    signature = sign(private_key, data)
    assert not verify(other_public, data, signature)
