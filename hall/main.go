package main

import (
	"common/config"
	"common/metrics"
	"context"
	"fmt"
	"framework/game"
	"hall/app"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "hall",
	Short: "hall 大厅相关的处理",
	Long:  `hall 大厅相关的处理`,
	Run: func(cmd *cobra.Command, args []string) {
	},
	PostRun: func(cmd *cobra.Command, args []string) {
	},
}

var (
	configFile    string
	gameConfigDir string
	serverId      string
)

func init() {
	// 设置默认值
	rootCmd.Flags().StringVar(&configFile, "config", "application.yml", "app config yml file")
	rootCmd.Flags().StringVar(&gameConfigDir, "gameDir", "../config", "game config dir")
	rootCmd.Flags().StringVar(&serverId, "serverId", "connector001", "app server id， required")
	_ = rootCmd.MarkFlagRequired("serverId")
}

//var configFile = flag.String("config", "application.yml", "config file")

func main() {
	// 1.加载配置
	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
	config.InnitConfig(configFile)
	game.InitConfig(gameConfigDir)
	fmt.Println(config.Conf)
	// 2.启动监控
	go func() {
		err := metrics.Serve(fmt.Sprintf("0.0.0.0:%d", config.Conf.MetricPort))
		if err != nil {
			panic(err)
		}
	}()
	// 3.连接 nats 服务 并进行订阅
	err := app.Run(context.Background(), serverId)
	if err != nil {
		log.Println(err)
		os.Exit(-1)
	}
}
