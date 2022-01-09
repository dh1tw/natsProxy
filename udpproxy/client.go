package udpproxy

import (
	"fmt"
	"sync"

	"github.com/nats-io/nats.go"
)

type Client struct {
	sync.Mutex
	nc           *nats.Conn
	udpListeners map[string]*UDPListener
}

func NewClient(nc *nats.Conn) *Client {
	return &Client{
		nc:           nc,
		udpListeners: map[string]*UDPListener{},
	}
}

func (cl *Client) AddUDPListener(udpAddr, alias string) error {
	cl.Lock()
	defer cl.Unlock()

	if _, duplicate := cl.udpListeners[alias]; duplicate {
		return fmt.Errorf("udp listner '%s' already exists (%s)", alias, udpAddr)
	}

	udpl, err := NewUDPListener(udpAddr, alias, cl.nc)
	if err != nil {
		return err
	}

	go udpl.Run()
	cl.udpListeners[alias] = udpl

	return nil
}

func (cl *Client) Close() {
	cl.Lock()
	defer cl.Unlock()

	for _, udpl := range cl.udpListeners {
		udpl.Close()
	}
}
