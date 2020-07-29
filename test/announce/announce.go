package announce

import (
	"errors"

	"github.com/testground/sdk-go/runtime"
)

// RunTest tests cluster-wise peer announcement.  It verifies that hosts are mutually
// aware of each others' presence.
func RunTest(runenv *runtime.RunEnv) (err error) {
	return errors.New("NOT IMPLEMENTED")
	// ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	// defer cancel()

	// client := sync.MustBoundClient(ctx, runenv)
	// defer client.Close()

	// switch client.MustSignalAndWait(ctx, sync.State("init"), runenv.TestInstanceCount) {
	// case 1:
	// 	return announceClient(ctx, runenv, client)
	// default:
	// 	return announceHost(ctx, runenv, client)
	// }
}

// func announceClient(ctx context.Context, runenv *runtime.RunEnv, client *sync.DefaultClient) error {
// 	ctx, cancel := context.WithCancel(ctx)
// 	defer cancel()

// 	runenv.RecordMessage("I am the client")
// 	defer client.MustSignalEntry(context.Background(), sync.State("done"))

// 	// Wait for at least one host to be available.  We're purposefully playing fast and
// 	// loose to test dynamic joining of new hosts to an existing cluster.
// 	b := client.MustBarrier(ctx, sync.State("ready"), 1)
// 	if err := waitBarrier(ctx, b); err != nil {
// 		return err
// 	}

// 	c, err := wwclient.Dial(ctx)
// 	if err != nil {
// 		return err
// 	}
// 	defer c.Close()

// 	topic, err := c.Join("")
// 	if err != nil {
// 		return err
// 	}
// 	defer topic.Close()

// 	sub, err := topic.Subscribe(ctx)
// 	if err != nil {
// 		return err
// 	}

// 	ps := make(map[peer.ID]struct{})
// 	for msg := range sub.C {
// 		if _, ok := ps[msg.GetFrom()]; ok {
// 			continue
// 		}

// 		runenv.RecordMessage("got entry for %s", msg.GetFrom())

// 		// loop until at least one message from all peers was found.
// 		if ps[msg.GetFrom()] = struct{}{}; len(ps) == runenv.TestInstanceCount-1 {
// 			break
// 		}
// 	}

// 	return nil
// }

// func announceHost(ctx context.Context, runenv *runtime.RunEnv, client *sync.DefaultClient) error {
// 	ctx, cancel := context.WithCancel(ctx)
// 	defer cancel()

// 	host, err := host.New(ctx)
// 	defer host.Shutdown()

// 	runenv.RecordMessage("%s ready", host.ID())

// 	client.MustSignalEntry(ctx, sync.State("ready"))   // tell client we're good to go
// 	<-client.MustBarrier(ctx, sync.State("done"), 1).C // wait for client to terminate

// 	return nil
// }

// func waitBarrier(ctx context.Context, b *sync.Barrier) (err error) {
// 	select {
// 	case err = <-b.C:
// 	case <-ctx.Done():
// 		err = ctx.Err()
// 	}

// 	return
// }
