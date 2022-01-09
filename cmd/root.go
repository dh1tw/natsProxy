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
	"os"

	"github.com/spf13/cobra"

	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "natsProxy",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringP("broker-url", "u", "localhost", "Broker URL")
	rootCmd.PersistentFlags().IntP("broker-port", "p", 4222, "Broker Port")
	rootCmd.PersistentFlags().StringP("password", "P", "", "NATS Password")
	rootCmd.PersistentFlags().StringP("username", "U", "", "NATS Username")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.natsProxy.yaml)")

	viper.BindPFlag("nats.broker-url", rootCmd.Flags().Lookup("broker-url"))
	viper.BindPFlag("nats.broker-port", rootCmd.Flags().Lookup("broker-port"))
	viper.BindPFlag("nats.password", rootCmd.Flags().Lookup("password"))
	viper.BindPFlag("nats.username", rootCmd.Flags().Lookup("username"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName(".natsProxy") // name of config file (without extension)
		viper.AddConfigPath("$HOME")      // adding home directory as first search path
		viper.AddConfigPath(".")
	}
	viper.AutomaticEnv() // read in environment variables that match
}
