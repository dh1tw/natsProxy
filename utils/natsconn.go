package utils

import (
	"log"

	"github.com/nats-io/nats.go"
)

var reconnectHandler = func(c *nats.Conn) {
	log.Printf("Reconnected to nats-server %s\n", c.ConnectedAddr())
}

var disconnectedHandler = func(c *nats.Conn) {
	log.Printf("Disconnected from nats-server %s\n", c.ConnectedAddr())
}

func NatsConn(url string) (*nats.Conn, error) {

	nc, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}

	nc.SetReconnectHandler(reconnectHandler)
	nc.SetDisconnectHandler(disconnectedHandler)

	if nc.IsConnected() {
		log.Println("Connected to nats-server", nc.ConnectedAddr())
	}

	return nc, err
}
