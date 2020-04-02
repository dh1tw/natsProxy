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
	"net"
	"os"
	"os/signal"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: proxyServer,
}

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.Flags().StringP("broker-url", "u", "localhost", "Broker URL")
	serverCmd.Flags().IntP("broker-port", "p", 4222, "Broker Port")
	serverCmd.Flags().StringP("password", "P", "", "NATS Password")
	serverCmd.Flags().StringP("username", "U", "", "NATS Username")
	serverCmd.Flags().StringToStringP("host-mapping", "x",
		map[string]string{
			"ctl":   "192.168.0.30:50001",
			"cat":   "192.168.0.30:50002",
			"sound": "192.168.0.30:50003"}, "alias to host:port mapping")
}

type ServerProxy struct {
	sync.Mutex
	nc    *nats.Conn
	conns map[string]*net.UDPConn
	sub   *nats.Subscription
}

func proxyServer(cmd *cobra.Command, args []string) {

	viper.BindPFlag("nats.broker-url", cmd.Flags().Lookup("broker-url"))
	viper.BindPFlag("nats.broker-port", cmd.Flags().Lookup("broker-port"))
	viper.BindPFlag("nats.password", cmd.Flags().Lookup("password"))
	viper.BindPFlag("nats.username", cmd.Flags().Lookup("username"))
	viper.BindPFlag("proxy.host-mapping", cmd.Flags().Lookup("host-mapping"))

	hostMapping := viper.GetStringMapString("proxy.host-mapping")

	// Workaround due to bug in viper see:
	// https://github.com/spf13/viper/issues/608
	// PR in the pipeline: https://github.com/spf13/viper/pull/874
	if len(hostMapping) == 0 {
		var err error
		hostMapping, err = cmd.Flags().GetStringToString("host-mapping")
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

	var reconnectHandler = func(c *nats.Conn) {
		log.Printf("Reconnected to nats-server %s\n", c.ConnectedAddr())
	}

	var disconnectedHandler = func(c *nats.Conn) {
		log.Printf("Disconnected from nats-server %s\n", c.ConnectedAddr())
	}

	natsClient.SetDisconnectHandler(disconnectedHandler)
	natsClient.SetReconnectHandler(reconnectHandler)

	if natsClient.IsConnected() {
		log.Printf("Connected to nats-server %s:%v\n",
			viper.GetString("nats.broker-url"),
			viper.GetString("nats.broker-port"))
	}

	sp := ServerProxy{
		nc:    natsClient,
		conns: make(map[string]*net.UDPConn),
	}

	var msgHandler = func(msg *nats.Msg) {

		connID := msg.Subject[6:16]
		alias := msg.Subject[24:]
		// log.Println(msg.Subject)

		sp.Lock()
		conn, found := sp.conns[connID]
		sp.Unlock()
		if !found {
			url, ok := hostMapping[alias]
			if !ok {
				return // unknown alias
			}
			conn, err = Dial(url)
			if err != nil {
				log.Println(err)
				return
			}
			log.Printf("proxying traffic from '%s'/NATS to %v/UDP (ID: %s)\n",
				alias,
				conn.RemoteAddr(),
				connID)
			sp.Lock()
			sp.conns[connID] = conn
			sp.Unlock()
			go sp.RunUDPServer(conn, connID, alias)
		}

		_, err = conn.Write(msg.Data)
		if err != nil {
			log.Println(err)
		}
	}

	sub, err := sp.nc.Subscribe("proxy.*.to.udp.*", msgHandler)
	if err != nil {
		log.Fatal(err)
	}
	sp.sub = sub

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c
}

func Dial(address string) (*net.UDPConn, error) {
	resolvedAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, resolvedAddr)
	if err != nil {
		return nil, err
	}
	// defer conn.Close()

	return conn, nil
}

func (sp *ServerProxy) RunUDPServer(c *net.UDPConn, connID, alias string) {

	buf := make([]byte, 1500)

	topic := fmt.Sprintf("proxy.%s.from.udp.%s", connID, alias)
	for {
		n, _, err := c.ReadFromUDP(buf)
		if err != nil {
			log.Println(err)
		}

		// if alias != "ctl" {
		// 	log.Printf("%s to NATS: %x", alias, buf[:n])
		// }
		// log.Printf("%4d bytes: %v\n", n, topic)
		if err := sp.nc.Publish(topic, buf[:n]); err != nil {
			log.Println(err)
		}
	}
}
