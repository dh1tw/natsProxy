package udpproxy

import (
	"log"
	"net"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

type UDPListener struct {
	sync.Mutex
	address *net.UDPAddr
	alias   string
	conn    *net.UDPConn
	conns   map[string]*UDPProxyConn
	nc      *nats.Conn
	// errorCh chan<- struct{}
	closeCh <-chan struct{}
}

func NewUDPListener(address, alias string, nc *nats.Conn) (*UDPListener, error) {

	resolvedAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", resolvedAddr)
	if err != nil {
		return nil, err
	}

	udpl := &UDPListener{
		address: resolvedAddr,
		alias:   alias,
		conn:    conn,
		conns:   make(map[string]*UDPProxyConn),
		nc:      nc,
		// errorCh: make(chan struct{}),
		closeCh: make(chan struct{}),
	}

	return udpl, nil
}

func (udpl *UDPListener) Close() {

	udpl.Lock()
	defer udpl.Unlock()
	for name, conn := range udpl.conns {
		delete(udpl.conns, name)
		conn.Close()
	}
}

func (udpl *UDPListener) Run() {

	log.Printf("Listening on %v/UDP\n", udpl.address.String())
	defer udpl.conn.Close()

	buf := make([]byte, 1500)
	for {
		select {
		case <-udpl.closeCh:
			for _, c := range udpl.conns {
				c.Close()
			}
			return
		default:
		}
		udpl.conn.SetReadDeadline(time.Now().Add(time.Second))
		n, addr, err := udpl.conn.ReadFromUDP(buf)
		if err != nil {
			if err, ok := err.(net.Error); ok && err.Timeout() {
				continue
			}
			log.Println(err)
		}
		udpl.Lock()
		conn, found := udpl.conns[addr.String()]
		udpl.Unlock()
		if !found {
			conn, err = NewConnection(addr, udpl.nc, udpl.alias)
			if err != nil {
				log.Println(err)
			}
			log.Printf("%v/UDP proxying traffic from %v/UDP to '%s'/NATS (ID: %s)\n",
				udpl.address.String(),
				addr.String(),
				udpl.alias,
				conn.ID())
			udpl.Lock()
			udpl.conns[addr.String()] = conn
			udpl.Unlock()
		}

		if err := udpl.nc.Publish(conn.Topic(), buf[:n]); err != nil {
			log.Println(err)
		}
	}
}
