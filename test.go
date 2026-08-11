package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	hosts, err := createHosts(3)
	if err != nil {
		log.Fatal(err)
	}
	defer closeHosts(hosts)

	pss := make([]*pubsub.PubSub, len(hosts))
	for i, h := range hosts {
		ps, err := pubsub.NewGossipSub(ctx, h)
		if err != nil {
			log.Fatal(err)
		}
		pss[i] = ps
	}

	if err := connectHosts(ctx, hosts); err != nil {
		log.Fatal(err)
	}

	topicName := "gossipsub-test"
	const messageText = "hello from node 0"

	topics := make([]*pubsub.Topic, len(pss))
	subs := make([]*pubsub.Subscription, len(pss))
	for i, ps := range pss {
		topic, err := ps.Join(topicName)
		if err != nil {
			log.Fatal(err)
		}
		sub, err := topic.Subscribe()
		if err != nil {
			log.Fatal(err)
		}
		topics[i] = topic
		subs[i] = sub
	}

	received := make(chan string, len(hosts)-1)
	var readers sync.WaitGroup
	for i := 1; i < len(subs); i++ {
		readers.Add(1)
		go func(index int, sub *pubsub.Subscription) {
			defer readers.Done()
			msg, err := sub.Next(ctx)
			if err != nil {
				log.Printf("node %d subscription stopped: %v", index, err)
				return
			}
			received <- fmt.Sprintf("node %d received: %s", index, string(msg.Data))
		}(i, subs[i])
	}

	time.Sleep(2 * time.Second)
	if err := topics[0].Publish(ctx, []byte(messageText)); err != nil {
		log.Fatal(err)
	}

	for i := 0; i < len(hosts)-1; i++ {
		select {
		case output := <-received:
			fmt.Println(output)
		case <-ctx.Done():
			log.Fatal("timed out waiting for gossipsub delivery")
		}
	}

	readers.Wait()
	fmt.Println("gossipsub test finished successfully")
}

func createHosts(count int) ([]host.Host, error) {
	hosts := make([]host.Host, 0, count)
	for i := 0; i < count; i++ {
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		if err != nil {
			for _, existing := range hosts {
				_ = existing.Close()
			}
			return nil, err
		}
		hosts = append(hosts, h)
	}

	return hosts, nil
}

func connectHosts(ctx context.Context, hosts []host.Host) error {
	for i := 1; i < len(hosts); i++ {
		target := peer.AddrInfo{ID: hosts[0].ID(), Addrs: hosts[0].Addrs()}
		hosts[i].Peerstore().AddAddrs(target.ID, target.Addrs, peerstore.PermanentAddrTTL)
		if err := hosts[i].Connect(ctx, target); err != nil {
			return err
		}
	}
	return nil
}

func closeHosts(hosts []host.Host) {
	for _, h := range hosts {
		_ = h.Close()
	}
}
