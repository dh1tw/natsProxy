/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// clientCmd represents the client command
var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: proxyClient,
}

func init() {
	rootCmd.AddCommand(clientCmd)
	clientCmd.Flags().StringP("broker-url", "u", "localhost", "Broker URL")
	clientCmd.Flags().IntP("broker-port", "p", 4222, "Broker Port")
	clientCmd.Flags().StringP("password", "P", "", "NATS Password")
	clientCmd.Flags().StringP("username", "U", "", "NATS Username")
	clientCmd.Flags().StringToStringP("port-mapping", "m", map[string]string{"60001": "ctl", "60002": "cat", "60003": "sound"}, "UDP ports to proxy")
	clientCmd.Flags().StringP("proxy-url", "y", "localhost", "interface on which the UDP proxy listens")
}

const maxBufferSize = 1024

type UDPProxyConn struct {
	conn         *net.UDPConn       // UDP Connection to the client
	subscription *nats.Subscription // NATS subscription on the client topic
	topic        string             // unique ID to identify this connection in the nats topic
}

func (u *UDPProxyConn) Close() {
	if u.conn != nil {
		_ = u.conn.Close()
	}
	if u.subscription != nil {
		_ = u.subscription.Unsubscribe()
	}
	return
}

func NewConnection(addr *net.UDPAddr, nc *nats.Conn, alias string) (*UDPProxyConn, error) {
	upc := new(UDPProxyConn)
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, err
	}
	upc.conn = conn
	id := randomString(10)
	upc.topic = fmt.Sprintf("proxy/%s/frmcli/udp/%s", id, alias)

	msgHandler := func(msg *nats.Msg) {
		// log.Printf("write (to %v) %x\n", c.ClientConn.RemoteAddr(), msg.Data)
		_, err := upc.conn.Write(msg.Data)
		if err != nil {
			log.Println(err)
		}
	}

	subTopic := fmt.Sprintf("proxy/%s/toCli/udp/%s", id, alias)
	sub, err := nc.Subscribe(subTopic, msgHandler)
	if err != nil {
		log.Println(err)
	}
	upc.subscription = sub

	return upc, nil
}

func (udpx *UDPProxyConn) Topic() string {
	return udpx.topic
}

type ClientProxy struct {
	sync.Mutex
	nc           *nats.Conn
	udpListeners map[string]*UDPListener
}

func proxyClient(cmd *cobra.Command, args []string) {
	viper.BindPFlag("nats.broker-url", cmd.Flags().Lookup("broker-url"))
	viper.BindPFlag("nats.broker-port", cmd.Flags().Lookup("broker-port"))
	viper.BindPFlag("nats.password", cmd.Flags().Lookup("password"))
	viper.BindPFlag("nats.username", cmd.Flags().Lookup("username"))
	viper.BindPFlag("proxy.port-mapping", cmd.Flags().Lookup("port-mapping"))
	viper.BindPFlag("proxy.url", cmd.Flags().Lookup("proxy-url"))

	portMapping := viper.GetStringMapString("proxy.port-mapping")

	// Workaround due to bug in viper see:
	// https://github.com/spf13/viper/issues/608
	// PR in the pipeline: https://github.com/spf13/viper/pull/874
	if len(portMapping) == 0 {
		var err error
		portMapping, err = cmd.Flags().GetStringToString("port-mapping")
		if err != nil {
			log.Fatal(err)
		}
	}

	natsURL := fmt.Sprintf("nats://%s:%s@%s:%d",
		viper.GetString("nats.username"),
		viper.GetString("nats.password"),
		viper.GetString("nats.broker-url"),
		viper.GetInt("nats.broker-port"),
	)

	natsClient, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatal(err)
	}

	if natsClient.IsConnected() {
		log.Printf("Connected to server %s:%v\n",
			viper.GetString("nats.broker-url"),
			viper.GetString("nats.broker-port"))
	}

	cp := ClientProxy{
		nc:           natsClient,
		udpListeners: make(map[string]*UDPListener),
	}

	// portMapping := viper.GetStringMapString("proxy.port-mapping")
	addr := viper.GetString("proxy.url")
	for port, alias := range portMapping {
		laddr := addr + ":" + string(port)
		udpl, err := NewUDPListener(laddr, alias, cp.nc)
		if err != nil {
			log.Fatal(err)
		}
		go udpl.Run()
		cp.udpListeners[laddr] = udpl
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	for _, udpl := range cp.udpListeners {
		udpl.Close()
	}
}

type UDPListener struct {
	sync.Mutex
	address *net.UDPAddr
	alias   string
	conn    *net.UDPConn
	conns   map[string]*UDPProxyConn
	nc      *nats.Conn
	errorCh chan<- struct{}
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
		errorCh: make(chan struct{}),
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

	log.Printf("proxying %v/UDP to '%s'/NATS)\n", udpl.address.String(), udpl.alias)
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
			log.Printf("Added incoming connection from %v/UDP\n", addr.String())
			udpl.Lock()
			udpl.conns[addr.String()] = conn
			udpl.Unlock()
		}

		// if alias != "ctl" {
		// 	log.Printf("%s to NATS: %x", alias, buf[:n])
		// }
		if err := udpl.nc.Publish(conn.Topic(), buf[:n]); err != nil {
			log.Println(err)
		}
	}
}

func randomString(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}
