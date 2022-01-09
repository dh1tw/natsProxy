package udpproxy

import (
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/nats-io/nats.go"
)

type Server struct {
	sync.Mutex
	nc          *nats.Conn
	conns       map[string]*udpServer
	hostMapping map[string]string
	sub         *nats.Subscription
}

func NewServer(nc *nats.Conn, hostMapping map[string]string) *Server {

	s := &Server{
		nc:          nc,
		conns:       map[string]*udpServer{},
		hostMapping: hostMapping,
	}

	sub, err := nc.Subscribe("proxy.*.to.udp.*", s.handleMsg)
	if err != nil {
		log.Fatal(err)
	}

	s.sub = sub
	return s
}

func (s *Server) Run() {

}

func (s *Server) handleMsg(msg *nats.Msg) {
	s.Lock()
	defer s.Unlock()

	// log.Println(msg.Subject)
	connID := msg.Subject[6:16]
	alias := msg.Subject[24:]

	conn, exists := s.conns[connID]

	if !exists {
		url, ok := s.hostMapping[alias]
		if !ok {
			log.Println("msg for unregistered alias dropped")
			return
		}

		udpSvr, err := newUdpServer(s.nc, url, connID, alias)
		if err != nil {
			log.Println(err)
			return
		}

		log.Printf("proxying traffic from '%s'/NATS to %v/UDP (ID: %s)\n",
			alias,
			udpSvr.udpAddr(),
			connID)
		s.conns[connID] = udpSvr
		go udpSvr.run()
	}

	_, err := conn.write(msg.Data)
	if err != nil {
		log.Println(err)
	}
}

type udpServer struct {
	buf       []byte
	natsTopic string
	natsConn  *nats.Conn
	udpConn   *net.UDPConn
}

func newUdpServer(natsConn *nats.Conn, udpAddr, connID, alias string) (*udpServer, error) {

	resolvedAddr, err := net.ResolveUDPAddr("udp", udpAddr)
	if err != nil {
		return nil, err
	}

	udpConn, err := net.DialUDP("udp", nil, resolvedAddr)
	if err != nil {
		return nil, err
	}

	u := &udpServer{
		buf:       make([]byte, 1500),
		udpConn:   udpConn,
		natsConn:  natsConn,
		natsTopic: fmt.Sprintf("proxy.%s.from.udp.%s", connID, alias),
	}

	return u, nil
}

func (u *udpServer) run() {

	for {
		n, _, err := u.udpConn.ReadFromUDP(u.buf)
		if err != nil {
			log.Println(err)
		}

		// forward what we received via UDP to the corresponding natsTopic
		if err := u.natsConn.Publish(u.natsTopic, u.buf[:n]); err != nil {
			log.Println(err)
		}
	}
}

func (u *udpServer) udpAddr() net.Addr {
	return u.udpConn.RemoteAddr()
}

func (u *udpServer) write(msg []byte) (int, error) {
	return u.udpConn.Write(msg)
}
