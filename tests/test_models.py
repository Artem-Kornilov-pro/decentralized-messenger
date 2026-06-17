"""Tests for core data models."""

from datetime import datetime, timezone

from decentralized_messenger.models.message import LogEntry, SignedMessage


def _make_message() -> SignedMessage:
    return SignedMessage(
        message_id="msg-1",
        chat_id="chat-1",
        sender_id="user-1",
        content=b"hello",
        timestamp=datetime(2026, 1, 1, tzinfo=timezone.utc),
        public_key=b"\x00" * 32,
        signature=b"\x00" * 64,
    )


def test_log_entry_computes_hash() -> None:
    entry = LogEntry(sequence=0, message=_make_message(), prev_hash="0" * 64)
    assert len(entry.entry_hash) == 64


def test_log_entry_hash_changes_with_sequence() -> None:
    msg = _make_message()
    e1 = LogEntry(sequence=0, message=msg, prev_hash="0" * 64)
    e2 = LogEntry(sequence=1, message=msg, prev_hash="0" * 64)
    assert e1.entry_hash != e2.entry_hash
