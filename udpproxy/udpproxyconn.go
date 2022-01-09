package udpproxy

import (
	"fmt"
	"log"
	"net"

	"github.com/dh1tw/natsProxy/utils"
	"github.com/nats-io/nats.go"
)

type UDPProxyConn struct {
	conn         *net.UDPConn       // UDP Connection to the client
	subscription *nats.Subscription // NATS subscription on the client topic
	id           string             // unique ID to identify this connection in the nats topic
	topic        string
}

func (u *UDPProxyConn) Close() {
	if u.conn != nil {
		_ = u.conn.Close()
	}
	if u.subscription != nil {
		_ = u.subscription.Unsubscribe()
	}
}

func NewConnection(addr *net.UDPAddr, nc *nats.Conn, alias string) (*UDPProxyConn, error) {
	upc := new(UDPProxyConn)
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, err
	}
	upc.conn = conn
	id := utils.RandomString(10)

	msgHandler := func(msg *nats.Msg) {
		// log.Printf("write (to %v) %x\n", c.ClientConn.RemoteAddr(), msg.Data)
		_, err := upc.conn.Write(msg.Data)
		if err != nil {
			log.Println(err)
		}
	}

	subTopic := fmt.Sprintf("proxy.%s.from.udp.%s", id, alias)
	sub, err := nc.Subscribe(subTopic, msgHandler)
	if err != nil {
		log.Println(err)
	}
	upc.subscription = sub
	upc.id = id
	upc.topic = fmt.Sprintf("proxy.%s.to.udp.%s", id, alias)

	return upc, nil
}

func (udpx *UDPProxyConn) Topic() string {
	return udpx.topic
}

func (udpx *UDPProxyConn) ID() string {
	return udpx.id
}
