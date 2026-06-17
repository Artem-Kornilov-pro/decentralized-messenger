"""Core data models for messages, log entries, and Merkle snapshots."""

from __future__ import annotations

import hashlib
from datetime import datetime
from typing import Self

from pydantic import BaseModel, Field, model_validator


class SignedMessage(BaseModel):
    """A chat message signed by the sender's Ed25519 key."""

    message_id: str
    chat_id: str
    sender_id: str
    content: bytes
    timestamp: datetime
    public_key: bytes
    signature: bytes


class LogEntry(BaseModel):
    """An immutable record in the append-only log."""

    sequence: int = Field(ge=0)
    message: SignedMessage
    prev_hash: str
    entry_hash: str = ""

    @model_validator(mode="after")
    def compute_hash(self) -> Self:
        if not self.entry_hash:
            payload = (
                f"{self.sequence}:{self.prev_hash}:"
                f"{self.message.message_id}:{self.message.timestamp.isoformat()}"
            ).encode()
            self.entry_hash = hashlib.sha256(payload).hexdigest()
        return self


class MerkleSnapshot(BaseModel):
    """A Merkle root snapshot created every 100 messages."""

    chat_id: str
    snapshot_index: int = Field(ge=0)
    from_sequence: int = Field(ge=0)
    to_sequence: int = Field(ge=0)
    merkle_root: str
    created_at: datetime
