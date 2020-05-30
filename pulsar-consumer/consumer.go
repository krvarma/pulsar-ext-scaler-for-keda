package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/apache/pulsar-client-go/pulsar"
)

func getEnv(key string, defvalue string) string {
	value := os.Getenv(key)

	if len(value) <= 0 {
		value = defvalue
	}

	return value
}

func main() {
	server := getEnv("PULSAR_SERVER", "pulsar://localhost:6650")
	topic := getEnv("PULSAR_TOPIC", "my-topic")
	subscriptionName := getEnv("PULSAR_SUBSCRIPTION_NAME", "first-subscription")

	log.Printf("Server: %v", server)
	log.Printf("Topic: %v", topic)
	log.Printf("Subscription: %v", subscriptionName)

	client, err := pulsar.NewClient(pulsar.ClientOptions{URL: server})
	if err != nil {
		log.Fatal(err)
	}

	defer client.Close()

	consumer, err := client.Subscribe(pulsar.ConsumerOptions{
		Topic:            topic,
		SubscriptionName: subscriptionName,
		Type:             pulsar.Shared,
	})

	if err != nil {
		log.Fatal(err)
	}

	defer consumer.Close()

	ctx := context.Background()

	// Listen indefinitely on the topic
	for {
		msg, err := consumer.Receive(ctx)
		if err != nil {
			log.Fatal(err)
		}

		// Do something with the message
		fmt.Printf("Message Received: %v\n", string(msg.Payload()))

		if err == nil {
			// Message processed successfully
			consumer.Ack(msg)
		} else {
			// Failed to process messages
			consumer.Nack(msg)
		}
	}
}
