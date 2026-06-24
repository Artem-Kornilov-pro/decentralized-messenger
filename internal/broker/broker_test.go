package broker

import "testing"

func TestInMemoryFanOut(t *testing.T) {
	b := NewInMemory()
	sub1, cancel1, err := b.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer cancel1()
	sub2, cancel2, err := b.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer cancel2()

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

func TestInMemoryCancelStopsDeliveryAndRemovesSubscriber(t *testing.T) {
	b := NewInMemory()
	sub, cancel, err := b.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	cancel()

	if len(b.subscribers) != 0 {
		t.Fatalf("expected subscriber to be removed, got %d remaining", len(b.subscribers))
	}
	if err := b.Publish(Event{Kind: EntryAppended}); err != nil {
		t.Fatal(err)
	}
	if _, ok := <-sub; ok {
		t.Fatal("expected channel to be closed after cancel")
	}
}

func TestNoopPublishSucceeds(t *testing.T) {
	var b Broker = Noop{}
	if err := b.Publish(Event{Kind: SnapshotCreated}); err != nil {
		t.Fatal(err)
	}
}

func TestNoopSubscribeNeverDelivers(t *testing.T) {
	var b Broker = Noop{}
	sub, cancel, err := b.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	select {
	case evt := <-sub:
		t.Fatalf("expected no event, got %+v", evt)
	default:
	}
}
