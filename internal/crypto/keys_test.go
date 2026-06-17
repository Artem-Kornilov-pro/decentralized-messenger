package crypto

import (
	"testing"
	"time"

	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/models"
)

func TestSignAndVerify(t *testing.T) {
	priv, pub, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("hello, messenger")
	sig := Sign(priv, data)
	if !Verify(pub, data, sig) {
		t.Fatal("valid signature rejected")
	}
}

func TestVerifyRejectsTamperedData(t *testing.T) {
	priv, pub, _ := GenerateKeyPair()
	sig := Sign(priv, []byte("original"))
	if Verify(pub, []byte("tampered"), sig) {
		t.Fatal("tampered data accepted")
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	priv, _, _ := GenerateKeyPair()
	_, otherPub, _ := GenerateKeyPair()
	sig := Sign(priv, []byte("data"))
	if Verify(otherPub, []byte("data"), sig) {
		t.Fatal("signature verified under wrong key")
	}
}

func TestSignMessageRoundTrip(t *testing.T) {
	priv, pub, _ := GenerateKeyPair()
	msg := models.SignedMessage{
		MessageID: "m1",
		ChatID:    "c1",
		SenderID:  "u1",
		Content:   []byte("hi"),
		Timestamp: time.Unix(0, 0).UTC(),
		PublicKey: pub,
	}
	signed := SignMessage(msg, priv)
	if !VerifyMessage(signed) {
		t.Fatal("signed message failed verification")
	}

	signed.Content = []byte("tampered")
	if VerifyMessage(signed) {
		t.Fatal("tampered message passed verification")
	}
}
