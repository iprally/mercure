package mercure

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	redisHost           = "localhost:6379"
	redisSubscriberSize = 100000
	redisChannel        = "channel"
)

func initialize(t *testing.T) *RedisTransport {
	t.Helper()

	transport, err := NewRedisTransport(slog.New(slog.DiscardHandler), redisHost, "", "", redisSubscriberSize, redisChannel, 0)
	if err != nil {
		t.Skipf("Redis is not available: %v", err)
	}

	// Flush leftover data from previous tests to ensure clean state.
	require.NoError(t, transport.client.FlushDB(context.Background()).Err())

	return transport
}

func TestRedisWaitListen(t *testing.T) {
	transport := initialize(t)
	defer transport.Close(context.Background())

	assert.Implements(t, (*Transport)(nil), transport)
	s := NewLocalSubscriber("", transport.logger, &TopicSelectorStore{})
	require.NoError(t, transport.AddSubscriber(context.Background(), s))

	var wg sync.WaitGroup
	wg.Go(func() {
		for range s.Receive() {
			t.Fail()
		}
	})

	s.Disconnect()
	wg.Wait()
}

func TestRedisDispatch(t *testing.T) {
	transport := initialize(t)
	defer transport.Close(context.Background())

	assert.Implements(t, (*Transport)(nil), transport)

	subscriber := NewLocalSubscriber("", transport.logger, &TopicSelectorStore{})
	subscriber.SetTopics([]string{"https://topics.local/topic", "https://topics.local/private"}, []string{"https://topics.local/private"})
	require.NoError(t, transport.AddSubscriber(context.Background(), subscriber))

	notSubscribed := &Update{Topics: []string{"not-subscribed"}}
	require.NoError(t, transport.Dispatch(context.Background(), notSubscribed))

	subscribedSkipped := &Update{Topics: []string{"https://topics.local/topic"}, Private: true}
	require.NoError(t, transport.Dispatch(context.Background(), subscribedSkipped))

	public := &Update{Topics: subscriber.SubscribedTopics}
	require.NoError(t, transport.Dispatch(context.Background(), public))
	assert.Equal(t, public, <-subscriber.Receive())
	private := &Update{Topics: subscriber.AllowedPrivateTopics, Private: true}
	require.NoError(t, transport.Dispatch(context.Background(), private))
	assert.Equal(t, private, <-subscriber.Receive())
}

func TestRedisClose(t *testing.T) {
	transport := initialize(t)

	require.NotNil(t, transport)
	defer transport.Close(context.Background())

	assert.Implements(t, (*Transport)(nil), transport)
	subscriber := NewLocalSubscriber("", transport.logger, &TopicSelectorStore{})
	subscriber.SetTopics([]string{"https://topics.local/topic"}, nil)
	require.NoError(t, transport.AddSubscriber(context.Background(), subscriber))
	require.NoError(t, transport.Close(context.Background()))
	require.Error(t, transport.AddSubscriber(context.Background(), subscriber))
	assert.Equal(t, transport.Dispatch(context.Background(), &Update{Topics: subscriber.SubscribedTopics}), ErrClosedTransport)
	_, ok := <-subscriber.out
	assert.False(t, ok)
}

func TestRedisHistoryReplay(t *testing.T) {
	transport := initialize(t)
	defer transport.Close(context.Background())

	topics := []string{"https://topics.local/topic"}

	// Publish event A — the subscriber's "last seen" event
	eventA := &Update{Topics: topics, Event: Event{Data: "event-a"}}
	require.NoError(t, transport.Dispatch(context.Background(), eventA))

	// Publish event B — subscriber is disconnected and misses this
	eventB := &Update{Topics: topics, Event: Event{Data: "event-b"}}
	require.NoError(t, transport.Dispatch(context.Background(), eventB))

	// Subscriber reconnects with Last-Event-ID = eventA.ID
	subscriber := NewLocalSubscriber(eventA.ID, transport.logger, &TopicSelectorStore{})
	subscriber.SetTopics(topics, nil)
	require.NoError(t, transport.AddSubscriber(context.Background(), subscriber))

	// Should receive event B from history replay
	received := <-subscriber.Receive()
	assert.Equal(t, "event-b", received.Data, "subscriber should receive the missed event from history")
}

func TestRedisHistoryReplayEvictedID(t *testing.T) {
	transport := initialize(t)
	defer transport.Close(context.Background())

	topics := []string{"https://topics.local/topic"}

	// Publish events while subscriber is disconnected
	eventA := &Update{Topics: topics, Event: Event{Data: "event-a"}}
	require.NoError(t, transport.Dispatch(context.Background(), eventA))

	eventB := &Update{Topics: topics, Event: Event{Data: "event-b"}}
	require.NoError(t, transport.Dispatch(context.Background(), eventB))

	// Wait for pub/sub messages to be consumed by the subscribe goroutine
	// before adding the subscriber, so they don't leak into the liveQueue.
	time.Sleep(100 * time.Millisecond)

	// Subscriber reconnects with a Last-Event-ID that has been evicted from the stream.
	// Like BoltDB, no history should be replayed when the ID is not found.
	subscriber := NewLocalSubscriber("urn:uuid:evicted-id", transport.logger, &TopicSelectorStore{})
	subscriber.SetTopics(topics, nil)
	require.NoError(t, transport.AddSubscriber(context.Background(), subscriber))

	// Dispatch a new live event to verify the subscriber still works
	eventC := &Update{Topics: topics, Event: Event{Data: "event-c"}}
	require.NoError(t, transport.Dispatch(context.Background(), eventC))

	received := <-subscriber.Receive()
	assert.Equal(t, "event-c", received.Data, "subscriber should only receive live events when Last-Event-ID is evicted")
}

func TestRedisConcurrent(t *testing.T) {
	transport1 := initialize(t)
	transport2 := initialize(t)
	transport3 := initialize(t)

	defer transport1.Close(context.Background())
	defer transport2.Close(context.Background())
	defer transport3.Close(context.Background())

	topics := []string{"https://topics.local/topic1", "https://topics.local/topic2", "https://topics.local/topic3"}

	const transport1EventsCount = 100

	const transport2EventsCount = 10

	const transport3EventsCount = 1

	const transport1SubscribersCount = 5

	const transport2SubscribersCount = 25

	const transport3SubscribersCount = 50

	wg := sync.WaitGroup{}
	wg.Add((transport1EventsCount + transport2EventsCount + transport3EventsCount) * (transport1SubscribersCount + transport2SubscribersCount + transport3SubscribersCount))

	counter1 := atomic.Int64{}
	counter2 := atomic.Int64{}
	counter3 := atomic.Int64{}

	transport1Subscribers := make([]*LocalSubscriber, 0, transport1SubscribersCount)
	transport2Subscribers := make([]*LocalSubscriber, 0, transport2SubscribersCount)
	transport3Subscribers := make([]*LocalSubscriber, 0, transport3SubscribersCount)

	defer func() {
		if recover() != nil {
			t.Logf("Counter 1 = %d\n", counter1.Load())
			t.Logf("Counter 2 = %d\n", counter2.Load())
			t.Logf("Counter 3 = %d\n", counter3.Load())
		}
	}()

	for range transport1SubscribersCount {
		subscriber := NewLocalSubscriber("", slog.New(slog.DiscardHandler), &TopicSelectorStore{})
		subscriber.SetTopics(topics, nil)
		transport1.AddSubscriber(context.Background(), subscriber)
		transport1Subscribers = append(transport1Subscribers, subscriber)
	}

	for range transport2SubscribersCount {
		subscriber := NewLocalSubscriber("", slog.New(slog.DiscardHandler), &TopicSelectorStore{})
		subscriber.SetTopics(topics, nil)
		transport2.AddSubscriber(context.Background(), subscriber)
		transport2Subscribers = append(transport2Subscribers, subscriber)
	}

	for range transport3SubscribersCount {
		subscriber := NewLocalSubscriber("", slog.New(slog.DiscardHandler), &TopicSelectorStore{})
		subscriber.SetTopics(topics, nil)
		transport3.AddSubscriber(context.Background(), subscriber)
		transport3Subscribers = append(transport3Subscribers, subscriber)
	}

	for range transport1EventsCount {
		update := Update{Topics: topics, Event: Event{Data: "test1"}}
		go transport1.Dispatch(context.Background(), &update)
	}

	for range transport2EventsCount {
		update := Update{Topics: topics, Event: Event{Data: "test2"}}
		go transport2.Dispatch(context.Background(), &update)
	}

	for range transport3EventsCount {
		update := Update{Topics: topics, Event: Event{Data: "test3"}}
		go transport3.Dispatch(context.Background(), &update)
	}

	for _, subscriber := range transport1Subscribers {
		go func() {
			for range subscriber.Receive() {
				counter1.Add(1)
				wg.Done()
			}
		}()
	}

	for _, subscriber := range transport2Subscribers {
		go func() {
			for range subscriber.Receive() {
				counter2.Add(1)
				wg.Done()
			}
		}()
	}

	for _, subscriber := range transport3Subscribers {
		go func() {
			for range subscriber.Receive() {
				counter3.Add(1)
				wg.Done()
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, int64(transport1EventsCount+transport2EventsCount+transport3EventsCount)*transport1SubscribersCount, counter1.Load())
	assert.Equal(t, int64(transport1EventsCount+transport2EventsCount+transport3EventsCount)*transport2SubscribersCount, counter2.Load())
	assert.Equal(t, int64(transport1EventsCount+transport2EventsCount+transport3EventsCount)*transport3SubscribersCount, counter3.Load())
}
