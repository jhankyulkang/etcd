package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/melbahja/goph"
	clientv3 "go.etcd.io/etcd/client/v3"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	MaxTxnOps = 128
)

type benchMergeReport struct {
	ScriptGet     int64 `json:"scriptGet"`
	ScriptPut     int64 `json:"scriptPut"`
	ScriptCleanUp int64 `json:"scriptCleanUp"`
	ScriptJoin    int64 `json:"scriptJoin"`
	ScriptRestart int64 `json:"scriptRestart"`

	Merge int64 `json:"merge"`
}

func benchmarkMerge(cfg config) {
	if len(cfg.Clusters) != 2 {
		log.Panicf("only support split benchmark with 2 clusters")
	}

	log.Printf("measure merge...")
	mergeCost := mergeBench(cfg)
	<-time.After(10 * time.Second)
	log.Printf("measure replicate...")
	getKey, putKey, cleanUp, join, restart := replicateBench(cfg)
	log.Printf("get: %v, put: %v, clean up: %v, join: %v, restart: %v\nmerge: %v\n",
		getKey.Milliseconds(), putKey.Milliseconds(), cleanUp.Milliseconds(), join.Milliseconds(), restart.Milliseconds(),
		mergeCost.Milliseconds())

	data, err := json.Marshal(benchMergeReport{
		ScriptGet:     getKey.Milliseconds(),
		ScriptPut:     putKey.Milliseconds(),
		ScriptCleanUp: cleanUp.Milliseconds(),
		ScriptJoin:    join.Milliseconds(),
		ScriptRestart: restart.Milliseconds(),
		Merge:         mergeCost.Milliseconds(),
	})
	if err != nil {
		log.Panicf("marshal benchmark report failed: %v\n", err)
	}
	if err = os.WriteFile(fmt.Sprintf("%v/bench-merge-%v-%v.json", cfg.Folder, cfg.Load, cfg.Repetition),
		data, 0666); err != nil {
		panic(fmt.Sprintf("write report json failed: %v", err))
	}
}

func replicateBench(cfg config) (getKey, putKey, cleanUp, join, restart time.Duration) {
	// connect to hosts
	sshClis := make([]map[string]*goph.Client, len(cfg.Clusters))
	password := os.Getenv(cfg.PasswordEnvVar)
	for idx, clr := range cfg.Clusters {
		sshClis[idx] = make(map[string]*goph.Client)
		for _, ep := range clr {
			host := strings.Split(strings.Replace(ep, "http://", "", 1), ":")[0]
			cli, err := goph.NewUnknown("kezhi", host, goph.Password(password))
			if err != nil {
				log.Panicf("connect to host %v failed: %v\n", host, err)
			}
			sshClis[idx][ep] = cli
			defer cli.Close()
		}
	}

	// start etcd servers
	wg := sync.WaitGroup{}
	etcdConfigs1 := parseEtcdConfigs(getClusterUrl(cfg.Clusters[0], 1))
	etcdConfigs2 := parseEtcdConfigs(getClusterUrl(cfg.Clusters[1], 1+len(cfg.Clusters[0])))
	for host, cli := range sshClis[0] {
		wg.Add(1)
		go func(cli *goph.Client, host string) {
			mustStartEtcd(cli, cfg.EtcdServerDir, etcdConfigs1[host])
			log.Printf("start etcd server on host %v\n", host)
			wg.Done()
		}(cli, host)
	}
	for host, cli := range sshClis[1] {
		wg.Add(1)
		go func(cli *goph.Client, host string) {
			mustStartEtcd(cli, cfg.EtcdServerDir, etcdConfigs2[host])
			log.Printf("start etcd server on host %v\n", host)
			wg.Done()
		}(cli, host)
	}
	wg.Wait()

	// prepare some load
	log.Printf("prepare load...")
	wg = sync.WaitGroup{}
	for i := 0; i < len(cfg.Clusters); i++ {
		wg.Add(1)
		go func(i int) {
			prepareLoad(cfg.Clusters[i][0], cfg.Load, cfg.Load*uint64(i))
			wg.Done()
		}(i)
	}
	wg.Wait()

	log.Printf("ready to start...")
	time.After(5 * time.Second)
	start := time.Now()

	// retrieve keys from second cluster and put to the first one
	// retrieve keys
	getCli := mustCreateClientWithTimeout(10*time.Minute, cfg.Clusters[1]...)
	kvs := make([][2][]byte, 0, cfg.Load)

	startIdx := cfg.Load
	endIdx := cfg.Load * 2
	endKey := strconv.FormatUint(endIdx, 10)
	for offset := startIdx; ; {
		resp, err := getCli.Do(context.TODO(), clientv3.OpGet(
			strconv.FormatUint(offset, 10), clientv3.WithRange(endKey)))
		if err != nil {
			log.Panicf("get keys failed: %v\n", err)
		}
		for _, kv := range resp.Get().Kvs {
			kvs = append(kvs, [2][]byte{kv.Key, kv.Value})
		}

		if resp.Get().More {
			offset += uint64(resp.Get().Count)
		} else {
			break
		}
	}
	getKeyTime := time.Now()

	// put keys
	putCli := mustCreateClientWithTimeout(10*time.Minute, cfg.Clusters[0]...)
	txn := putCli.Txn(context.TODO())
	ops := make([]clientv3.Op, 0, MaxTxnOps)
	for i := 0; i < len(kvs); i++ {
		ops = append(ops, clientv3.OpPut(string(kvs[i][0]), string(kvs[i][1])))
		if len(ops) == MaxTxnOps || i == len(kvs)-1 {
			if _, err := txn.Then(ops...).Commit(); err != nil {
				log.Printf("commit batch ended at %v failed: %v\n", i, err)
			}
			txn = putCli.Txn(context.TODO())
			ops = ops[:0]
		}
	}
	putKeyTime := time.Now()

	// stop second cluster and clean up
	cleanWithWait(sshClis[1], etcdConfigs2, [][]string{cfg.Clusters[1]}, cfg.EtcdServerDir, false)
	cleanUpTime := time.Now()

	// add second clusters to the first one
	if _, err := putCli.MemberJoint(context.TODO(), cfg.Clusters[1], nil); err != nil {
		log.Panicf("add members failed: %v\n", err)
	}
	joinTime := time.Now()

	// restart second cluster
	wg = sync.WaitGroup{}
	joinClusterUrl := getClusterUrl(append(cfg.Clusters[0], cfg.Clusters[1]...), 1)
	for host, cli := range sshClis[1] {
		wg.Add(1)
		go func(cli *goph.Client, host string) {
			joinConfig := etcdConfigs2[host]
			joinConfig.initialCluster = joinClusterUrl
			joinConfig.initialClusterState = "existing"
			mustStartEtcd(cli, cfg.EtcdServerDir, joinConfig)
			log.Printf("start etcd server on host %v\n", host)
			wg.Done()
		}(cli, host)
	}
	wg.Wait()
	restartTime := time.Now()

	putCli.Close()
	getCli.Close()

	clean(sshClis[0], etcdConfigs1, [][]string{cfg.Clusters[0]}, cfg.EtcdServerDir)
	clean(sshClis[1], etcdConfigs2, [][]string{cfg.Clusters[1]}, cfg.EtcdServerDir)

	return getKeyTime.Sub(start), putKeyTime.Sub(getKeyTime), cleanUpTime.Sub(putKeyTime),
		joinTime.Sub(cleanUpTime), restartTime.Sub(joinTime)
}

func mergeBench(cfg config) time.Duration {
	// connect to hosts
	sshClis := make([]map[string]*goph.Client, len(cfg.Clusters))
	password := os.Getenv(cfg.PasswordEnvVar)
	for idx, clr := range cfg.Clusters {
		sshClis[idx] = make(map[string]*goph.Client)
		for _, ep := range clr {
			host := strings.Split(strings.Replace(ep, "http://", "", 1), ":")[0]
			cli, err := goph.NewUnknown("kezhi", host, goph.Password(password))
			if err != nil {
				log.Panicf("connect to host %v failed: %v\n", host, err)
			}
			sshClis[idx][ep] = cli
			defer cli.Close()
		}
	}

	// start etcd servers
	wg := sync.WaitGroup{}
	etcdConfigs1 := parseEtcdConfigs(getClusterUrl(cfg.Clusters[0], 1))
	etcdConfigs2 := parseEtcdConfigs(getClusterUrl(cfg.Clusters[1], 1+len(cfg.Clusters[0])))
	for host, cli := range sshClis[0] {
		wg.Add(1)
		go func(cli *goph.Client, host string) {
			mustStartEtcd(cli, cfg.EtcdServerDir, etcdConfigs1[host])
			log.Printf("start etcd server on host %v\n", host)
			wg.Done()
		}(cli, host)
	}
	for host, cli := range sshClis[1] {
		wg.Add(1)
		go func(cli *goph.Client, host string) {
			mustStartEtcd(cli, cfg.EtcdServerDir, etcdConfigs2[host])
			log.Printf("start etcd server on host %v\n", host)
			wg.Done()
		}(cli, host)
	}
	wg.Wait()

	// prepare some load
	log.Printf("prepare load...")
	wg = sync.WaitGroup{}
	for i := 0; i < len(cfg.Clusters); i++ {
		wg.Add(1)
		go func(i int) {
			prepareLoad(cfg.Clusters[i][0], cfg.Load, cfg.Load*uint64(i))
			wg.Done()
		}(i)
	}
	wg.Wait()

	mlist := getMergeMemberList([]string{cfg.Clusters[1][0]})

	log.Printf("ready to start...")
	time.After(1 * time.Minute)

	start := time.Now()
	for {
		cli := mustCreateClientWithTimeout(10*time.Minute, cfg.Clusters[0][0])
		defer cli.Close()

		_, err := cli.MemberMerge(context.TODO(), mlist)
		if err != nil {
			if strings.Contains(err.Error(), "error reading from server: EOF") {
				continue
			}
			log.Panicf("merge failed: %v\n", err)
		}
		break
	}
	cost := time.Since(start)

	clean(sshClis[0], etcdConfigs1, [][]string{cfg.Clusters[0]}, cfg.EtcdServerDir)
	clean(sshClis[1], etcdConfigs2, [][]string{cfg.Clusters[1]}, cfg.EtcdServerDir)

	return cost
}
