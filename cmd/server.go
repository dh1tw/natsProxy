package cmd

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"

	"github.com/dh1tw/natsProxy/udpproxy"
	"github.com/dh1tw/natsProxy/utils"
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
	serverCmd.Flags().StringToStringP("host-mapping", "x",
		map[string]string{
			"ctl":   "192.168.0.30:50001",
			"cat":   "192.168.0.30:50002",
			"sound": "192.168.0.30:50003"}, "NATS alias to host:port mapping")
}

type ServerProxy struct {
	sync.Mutex
	nc    *nats.Conn
	conns map[string]*net.UDPConn
	sub   *nats.Subscription
}

func proxyServer(cmd *cobra.Command, args []string) {

	// Try to read config file
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	} else {
		if strings.Contains(err.Error(), "Not Found in") {
			fmt.Println("no config file found")
		} else {
			fmt.Println("Error parsing config file", viper.ConfigFileUsed())
			fmt.Println(err)
			os.Exit(1)
		}
	}

	viper.BindPFlag("proxy.host-mapping", cmd.Flags().Lookup("host-mapping"))

	hostMapping := viper.GetStringMapString("proxy.host-mapping")

	natsURL := fmt.Sprintf("nats://%s:%s@%s:%d",
		viper.GetString("nats.username"),
		viper.GetString("nats.password"),
		viper.GetString("nats.broker-url"),
		viper.GetInt("nats.broker-port"),
	)

	natsConn, err := utils.NatsConn(natsURL)
	if err != nil {
		log.Fatal(err)
	}

	server := udpproxy.NewServer(natsConn, hostMapping)
	server.Run()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c
}
