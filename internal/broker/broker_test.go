package broker

import "testing"

func TestInMemoryFanOut(t *testing.T) {
	b := NewInMemory()
	sub1 := b.Subscribe()
	sub2 := b.Subscribe()

	evt := Event{Kind: EntryAppended, ChatID: "c1", Sequence: 0, EntryHash: "abc"}
	if err := b.Publish(evt); err != nil {
		t.Fatal(err)
	}

	for i, sub := range []<-chan Event{sub1, sub2} {
		select {
		case got := <-sub:
			if got != evt {
				t.Fatalf("subscriber %d got %+v", i, got)
			}
		default:
			t.Fatalf("subscriber %d received no event", i)
		}
	}
}

func TestNoopPublishSucceeds(t *testing.T) {
	var b Broker = Noop{}
	if err := b.Publish(Event{Kind: SnapshotCreated}); err != nil {
		t.Fatal(err)
	}
}
