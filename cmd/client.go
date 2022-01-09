package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/dh1tw/natsProxy/udpproxy"
	"github.com/dh1tw/natsProxy/utils"
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
	clientCmd.Flags().StringToStringP("port-mapping", "m",
		map[string]string{
			"0.0.0.0:60001":   "ctl",
			":60002":          "cat",
			"127.0.0.1:60003": "sound"}, "host:port to NATS alias mapping")
}

func proxyClient(cmd *cobra.Command, args []string) {

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

	viper.BindPFlag("proxy.port-mapping", cmd.Flags().Lookup("port-mapping"))

	portMapping := viper.GetStringMapString("proxy.port-mapping")

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

	client := udpproxy.NewClient(natsConn)

	for udpAddr, alias := range portMapping {
		err := client.AddUDPListener(udpAddr, alias)
		if err != nil {
			log.Fatalf("%v", err)
		}
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	client.Close()
}
