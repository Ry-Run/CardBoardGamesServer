package game

import (
	"common/logs"
	"fmt"
	"log"
	"os"
	"path"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

var Conf *Config

const (
	gameConfig = "gameConfig.json"
	servers    = "servers.json"
)

type Config struct {
	GameConfig  map[string]GameConfigValue `json:"gameConfig"`
	ServersConf ServersConf                `json:"serversConf"`
}
type ServersConf struct {
	Nats       NatsConfig         `json:"nats"`
	Connector  []*ConnectorConfig `json:"connector"`
	Servers    []*ServersConfig   `json:"servers"`
	TypeServer map[string][]*ServersConfig
}

type ServersConfig struct {
	ID               string `json:"id"`
	ServerType       string `json:"serverType"`
	HandleTimeOut    int    `json:"handleTimeOut"`
	RPCTimeOut       int    `json:"rpcTimeOut"`
	MaxRunRoutineNum int    `json:"maxRunRoutineNum"`
}

type ConnectorConfig struct {
	ID         string `json:"id"`
	Host       string `json:"host"`
	ClientPort int    `json:"clientPort"`
	Frontend   bool   `json:"frontend"`
	ServerType string `json:"serverType"`
}
type NatsConfig struct {
	Url string `json:"url"`
}

type GameConfigValue map[string]any

// 读取指定文件夹下的所有配置
func InitConfig(configDir string) {
	Conf = new(Config)
	dir, err := os.ReadDir(configDir)
	if err != nil {
		logs.Fatal("read configDir failed: err=%v", configDir)
	}
	for _, file := range dir {
		if file.Name() == gameConfig {
			configFile := path.Join(configDir, file.Name())
			readGameConfig(configFile)
		}

		if file.Name() == servers {
			configFile := path.Join(configDir, file.Name())
			readServersConfig(configFile)
		}
	}
}

func readGameConfig(configFile string) {
	var gameConfig = make(map[string]GameConfigValue)

	v := viper.New()
	v.SetConfigFile(configFile)
	// 监听配置变化
	v.WatchConfig()
	v.OnConfigChange(func(in fsnotify.Event) {
		log.Println("GameConfig 配置文件变更")
		err := v.Unmarshal(&gameConfig)
		if err != nil {
			panic(fmt.Errorf("GameConfig viper unmarshal change config data: cast exception, err=%v \n", err))
		}
		Conf.GameConfig = gameConfig
	})
	err := v.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("GameConfig 读取配置出错, err=%v \n", err))
	}
	// 解析
	err = v.Unmarshal(&gameConfig)
	if err != nil {
		panic(fmt.Errorf("GameConfig viper unmarshal config data: cast exception, err=%v \n", err))
	}
	Conf.GameConfig = gameConfig
}

func readServersConfig(configFile string) {
	var serversConf ServersConf

	v := viper.New()
	v.SetConfigFile(configFile)
	// 监听配置变化
	v.WatchConfig()
	v.OnConfigChange(func(in fsnotify.Event) {
		log.Println("serversConf 配置文件变更")
		err := v.Unmarshal(&serversConf)
		if err != nil {
			panic(fmt.Errorf("serversConf viper unmarshal change config data: cast exception, err=%v \n", err))
		}
		Conf.ServersConf = serversConf
	})
	err := v.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("serversConf 读取配置出错, err=%v \n", err))
	}
	// 解析
	err = v.Unmarshal(&serversConf)
	if err != nil {
		panic(fmt.Errorf("serversConf viper unmarshal config data: cast exception, err=%v \n", err))
	}
	Conf.ServersConf = serversConf
	typeServerConfig()
}

func typeServerConfig() {
	if len(Conf.ServersConf.Servers) > 0 {
		if Conf.ServersConf.TypeServer == nil {
			Conf.ServersConf.TypeServer = make(map[string][]*ServersConfig)
		}
		for _, v := range Conf.ServersConf.Servers {
			if Conf.ServersConf.TypeServer[v.ServerType] == nil {
				Conf.ServersConf.TypeServer[v.ServerType] = make([]*ServersConfig, 0)
			}
			Conf.ServersConf.TypeServer[v.ServerType] = append(Conf.ServersConf.TypeServer[v.ServerType], v)
		}
	}
}

func (c *Config) GetConnector(serverId string) *ConnectorConfig {
	for _, config := range c.ServersConf.Connector {
		if config.ID == serverId {
			return config
		}
	}
	return nil
}
