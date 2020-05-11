package main

import (
	"context"
	"encoding/binary"
	"io"
	"time"

	wwclient "github.com/lthibault/wetware/pkg/client"
	"github.com/lthibault/wetware/pkg/server"
	"github.com/lthibault/ww-test-plans/testutil"
	"github.com/pkg/errors"
	"github.com/testground/sdk-go/runtime"
)

// Heartbeat tests the sequence numbers on heartbeat messages.  They must be
// moonotonically increasing.
func Heartbeat(runenv *runtime.RunEnv) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	h, err := startHost(ctx, runenv)
	if err != nil {
		return err
	}
	defer h.Close()

	c, err := wwclient.Dial(ctx,
		wwclient.WithLogger(testutil.ZapLogger(runenv)))
	if err != nil {
		return err
	}
	defer c.Close()

	topic, err := c.Join("")
	if err != nil {
		return err
	}
	defer topic.Close()

	subCtx, cancelSubscription := context.WithCancel(ctx)
	defer cancelSubscription()

	sub, err := topic.Subscribe(subCtx)
	if err != nil {
		return err
	}

	seq := make([]uint64, 0, 10)
	for msg := range sub.C {
		if seq = append(seq, binary.BigEndian.Uint64(msg.GetSeqno())); len(seq) == 10 {
			cancelSubscription()
		}
	}

	var prev uint64
	for _, i := range seq {
		if prev >= i {
			err = errors.Errorf("sequence violation: %d >= %d", prev, i)
			runenv.RecordFailure(err)
			break
		}
	}

	return err
}

func startHost(ctx context.Context, runenv *runtime.RunEnv) (io.Closer, error) {
	ch := make(chan error, 1)
	host := server.New(
		server.WithLogger(testutil.ZapLogger(runenv)),
		server.WithTTL(time.Millisecond*10))

	go func() {
		defer close(ch)
		ch <- host.Start()
	}()

	select {
	case err := <-ch:
		return host, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
