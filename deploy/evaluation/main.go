package main

import (
	"fmt"
	clientv3 "go.etcd.io/etcd/client/v3"
	"gopkg.in/yaml.v3"
	"os"
	"time"
)

const ConfigFile = "./config.yaml"

type config struct {
	Type   string
	Folder string

	Clusters [][]string
	Before   uint64
	After    uint64
	Threads  uint64
	Load     uint64

	Repetition uint64

	User           string
	PasswordEnvVar string
	EtcdServerDir  string
	EtcdctlPath    string
	EtcdutlPath    string
}

func main() {
	var cfg config
	if data, err := os.ReadFile(ConfigFile); err == nil {
		err = yaml.Unmarshal(data, &cfg)
		if err != nil {
			panic("unmarshal config file failed: " + err.Error())
		}
	} else {
		panic("read config file failed: " + err.Error())
	}

	switch cfg.Type {
	case "add-performance":
		addPerformanceLOOP(cfg)
	default:
		panic(fmt.Sprintf("unknown type: %v", cfg.Type))
	}
}

func mustCreateClient(endpoint ...string) *clientv3.Client {
	return mustCreateClientWithTimeout(1*time.Minute, endpoint...)
}

func mustCreateClientWithTimeout(timeout time.Duration, endpoint ...string) *clientv3.Client {
	cli, err := clientv3.New(clientv3.Config{Endpoints: endpoint, DialKeepAliveTimeout: timeout})
	if err != nil {
		panic(fmt.Sprintf("create client for endpoint %v failed: %v", endpoint, err))
	}
	return cli
}
