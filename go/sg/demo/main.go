package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"time"

	"github.com/celrenheit/sandflake"
	"github.com/celrenheit/sandglass/client"
	"github.com/celrenheit/sandglass-grpc/go/sgproto"
)

func main() {
	addr := flag.String("addrs", "", "GRPC addresses of sandglass cluster")

	flag.Parse()

	c, err := client.New(client.WithAddresses(*addr))
	if err != nil {
		panic(err)
	}

	defer c.Close()

	topic := "payments"

	partitions, err := c.ListPartitions(context.Background(), topic)
	if err != nil {
		panic(err)
	}

	start := time.Now()
	partition := partitions[0]
	var gen sandflake.Generator

	msgCh, errCh := c.ProduceMessageCh(context.Background())
	for i := 0; i < 1e3; i++ {
		msg := &sgproto.Message{
			Topic:     topic,
			Partition: partition,
			Offset:    gen.Next(),
			Value:     randomBytes(1e4),
		}

		select {
		case msgCh <- msg:
		case err := <-errCh:
			panic(err)
		}
	}
	close(msgCh)
	fmt.Println("took", time.Since(start))
	start = time.Now()

	fmt.Println("consuming...")
	ctx := context.Background()
	consumer := c.NewConsumer(topic, partition, "group", "consumer1")

	consumeCh, err := consumer.Consume(ctx)
	if err != nil {
		panic(err)
	}

	n := 0
	var msg *sgproto.Message
	offsets := []sandflake.ID{}
	for msg = range consumeCh {
		offsets = append(offsets, msg.Offset)
		n++
		if len(offsets) == 1000 {
			err = consumer.AcknowledgeMessages(context.Background(), offsets)
			if err != nil {
				panic(err)
			}

			offsets = offsets[:0]
		}
	}

	err = consumer.AcknowledgeMessages(context.Background(), offsets)
	if err != nil {
		panic(err)
	}

	if msg != nil {
		fmt.Println("committing", msg.Offset)
		err := consumer.Commit(ctx, msg)
		if err != nil {
			panic(err)
		}
	}

	fmt.Println("took", time.Since(start), "for", n)
	if n > 0 {
		fmt.Println("each took", time.Duration(time.Since(start).Nanoseconds()/int64(n)))
	}
}

func randomBytes(n int) []byte {
	data := make([]byte, n)
	rand.Read(data)
	return data
}
