package main

import (
	//"bufio"
	"context"
	"encoding/json"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	//"bytes"
	//"encoding/json"
	"fmt"
	"github.com/melbahja/goph"
	//"go.etcd.io/etcd/api/v3/etcdserverpb"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	//"io"
	//"os"
	//v3 "go.etcd.io/etcd/client/v3"
	"log"
	//"strconv"
	"time"
)

type Config struct {
	Auth           goph.Auth
	User           string
	Addr           string
	Port           uint
	Timeout        time.Duration
	Callback       ssh.HostKeyCallback
	BannerCallback ssh.BannerCallback
}
type addReport struct {
	Start int64 `json:"start"`
	Issue int64 `json:"issue"`

	//	Queries []add_query `json:"queries"` // queries send to original leader cluster

	//	Observes []observe `json:"observes"` // observe on non-original-leader cluster

	Leader addMeasure `json:"leader"` // measure collected from origin leader
	//	Measures []splitMeasure `json:"measures"` // measures collected from non-original-leader cluster
}

type addMeasure struct {
	AddEnter    int64 `json:"AddEnter"`
	AddLeave    int64 `json:"AddLeave"`
	LeaderElect int64 `json:"leaderElect"`
}

type add_observe struct {
	Observe int64       `json:"observe"` // unix microsecond timestamp on observing the leader
	Queries []add_query `json:"queries"`
}

type add_query struct {
	Start   int64 `json:"start"`   // unix microsecond timestamp
	Latency int64 `json:"latency"` // in microsecond
}

func addPerformance_B(cfg config) {
	// get members' id and find the leader before add
	clusterIds := make([][]uint64, len(cfg.Clusters))
	leaderId := uint64(0)
	leaderEp := ""
	leaderClrIdx := -1
	for idx, clr := range cfg.Clusters {
		clusterIds[idx] = make([]uint64, 0, len(clr))
		for _, ep := range clr {
			cli := mustCreateClient(ep)
			log.Printf(ep)
			resp, err := cli.Status(context.TODO(), ep)
			if err != nil {
				panic(fmt.Sprintf("get status for endpoint %v failed: %v", ep, err.Error()))
			}
			if leaderId != 0 && leaderId != resp.Leader {
				panic(fmt.Sprintf("leader not same: %v and %v", leaderId, resp.Leader))
			}

			leaderId = resp.Leader
			if resp.Header.MemberId == leaderId {
				leaderEp = ep
				leaderClrIdx = idx
			}

			clusterIds[idx] = append(clusterIds[idx], resp.Header.MemberId)
			if err = cli.Close(); err != nil {
				panic(err)
			}
		}
	}
	if leaderEp == "" || leaderClrIdx == -1 {
		panic("leader not found")
	} else {
		log.Printf("found leader %v at endpoint %v\n", leaderId, leaderEp)
	}

	// add memeber
	log.Printf("ready to start")

	//start := time.Now()

	list := getAddMemberList(clusterIds)
	for _, mem := range list {
		addCli := mustCreateClient(leaderEp)
		stopCh := make(chan struct{})
		addDoneCh := make(chan struct{})
		print(mem)
		var l [1]string
		l[0] = mem
		var memb []string = l[0:]
		log.Print(<-time.After(time.Duration(cfg.Before) * time.Second))

		// issue add
		//issue := time.Now()
		ctx, _ := context.WithTimeout(context.Background(), time.Minute*5)
		if _, err := addCli.MemberAdd(ctx, memb, 0); err != nil {
			panic(fmt.Sprintf("add failed: %v", err))
		}
		close(addDoneCh)

		// after add
		log.Print(<-time.After(time.Duration(cfg.After) * time.Second))
		close(stopCh)
		addCli.Close()
	}

	log.Printf("collect results...")
}
func addPerformance5(cfg config) {
	log.Printf("here")
	pKey := []byte(`-----BEGIN OPENSSH PRIVATE KEY-----
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
-----END OPENSSH PRIVATE KEY-----`)
		clusterIds := make([][]uint64, len(cfg.Clusters))
		leaderId := uint64(0)
		leaderEp := ""
		leaderClrIdx := -1
		var nm = []string{"http://192.168.0.99:2380"}
		host := "192.168.0.101:22"
		var cmd = "cd ~/etcdjk/server && nohup ./server --data-dir=data.etcd.5 --name=5 --heartbeat-interval=50 --election-timeout=1000 --listen-client-urls=http://192.168.0.99:2379 --advertise-client-urls=http://192.168.0.99:2379 --initial-advertise-peer-urls=http://192.168.0.99:2380 --listen-peer-urls=http://192.168.0.99:2380 --initial-cluster=1=http://192.168.0.140:2380,2=http://192.168.0.218:2380,3=http://192.168.0.245:2380,4=http://192.168.0.101:2380,5=http://192.168.0.99:2380 --initial-cluster-state=existing --pre-vote=false --log-level=panic > etcd.5.out 2>&1 &"
		for idx, clr := range cfg.Clusters {
			clusterIds[idx] = make([]uint64, 0, len(clr))
			for _, ep := range clr {
				cli := mustCreateClient(ep)
				resp, err := cli.Status(context.TODO(), ep)
				if err != nil {
					panic(fmt.Sprintf("get status for endpoint %v failed: %v", ep, err.Error()))
				}
				if leaderId != 0 && leaderId != resp.Leader {
					panic(fmt.Sprintf("leader not same: %v and %v", leaderId, resp.Leader))
				}
				leaderId = resp.Leader
				if resp.Header.MemberId == leaderId {
					leaderEp = ep
					leaderClrIdx = idx
				}
				clusterIds[idx] = append(clusterIds[idx], resp.Header.MemberId)
				if err = cli.Close(); err != nil {
					panic(err)
				}
			}
		}
		if leaderEp == "" || leaderClrIdx == -1 {
			panic("leader not found")
		} else {
			log.Printf("found leader %v at endpoint %v\n", leaderId, leaderEp)
		}
		stopCh := make(chan struct{})
		addDoneCh := make(chan struct{})
		
		// add memeber
		log.Printf("ready to start: ")
		addCli := mustCreateClient(leaderEp)
		start := time.Now()
		ctx, _ := context.WithTimeout(context.Background(), time.Minute*5)
		if _, err := addCli.MemberAdd(ctx, nm, 0); err != nil {
			panic(fmt.Sprintf("add failed: %v", err))
		}
		log.Print(time.Since(start))
		close(addDoneCh)
		close(stopCh)
		addCli.Close()

		user := "ubuntu"
	
		var err error
		var signer ssh.Signer

		signer, err = ssh.ParsePrivateKey(pKey)
		if err != nil {
			fmt.Println(err.Error())
		}

		var hostkeyCallback ssh.HostKeyCallback
		hostkeyCallback, err = knownhosts.New("/home/ubuntu/.ssh/known_hosts")
		if err != nil {
			fmt.Println(err.Error())
		}

		conf := &ssh.ClientConfig{
			User:            user,
			HostKeyCallback: hostkeyCallback,
			Auth: []ssh.AuthMethod{
				ssh.Password("joshuakang"),
				ssh.PublicKeys(signer),
			},
		}

		var conn *ssh.Client

		conn, err = ssh.Dial("tcp", host, conf)
		if err != nil {
			fmt.Println(err.Error())
		}
		 
		session, err := conn.NewSession()
		if err != nil {
			fmt.Print(err)
		}

		err = session.Run(cmd)
		conn.Close()
		session.Close()
}

func addPerformanceLOOP(cfg config) { //LOOP
	pKey := []byte(`-----BEGIN OPENSSH PRIVATE KEY-----
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
-----END OPENSSH PRIVATE KEY-----`)
	var member_list = []string{
		"192.168.0.124", 
		"192.168.0.154"}
	var cmd_list = []string{
		"cd ~/etcd/server && nohup ./server --data-dir=data.etcd.4 --name=4 --heartbeat-interval=50 --election-timeout=1000 --listen-client-urls=http://192.168.0.124:2379 --advertise-client-urls=http://192.168.0.124:2379 --initial-advertise-peer-urls=http://192.168.0.124:2380 --listen-peer-urls=http://192.168.0.124:2380 --initial-cluster=1=http://192.168.0.32:2380,2=http://192.168.0.40:2380,3=http://192.168.0.86:2380,4=http://192.168.0.124:2380 --initial-cluster-state=existing --pre-vote=false --log-level=panic > etcd.4.out 2>&1 &",
		"cd ~/etcd/server && nohup ./server --data-dir=data.etcd.5 --name=5 --heartbeat-interval=50 --election-timeout=1000 --listen-client-urls=http://192.168.0.154:2379 --advertise-client-urls=http://192.168.0.154:2379 --initial-advertise-peer-urls=http://192.168.0.154:2380 --listen-peer-urls=http://192.168.0.154:2380 --initial-cluster=1=http://192.168.0.32:2380,2=http://192.168.0.40:2380,3=http://192.168.0.86:2380,4=http://192.168.0.124:2380,5=http://192.168.0.154:2380 --initial-cluster-state=existing --pre-vote=false --log-level=panic > etcd.5.out 2>&1 &"}
	//var member_list = []string{"192.168.0.154"}
	//var cmd_list = []string{"cd ~/etcd/server && nohup ./server --data-dir=data.etcd.5 --name=5 --heartbeat-interval=50 --election-timeout=1000 --listen-client-urls=http://192.168.0.154:2379 --advertise-client-urls=http://192.168.0.154:2379 --initial-advertise-peer-urls=http://192.168.0.154:2380 --listen-peer-urls=http://192.168.0.154:2380 --initial-cluster=1=http://192.168.0.32:2380,2=http://192.168.0.40:2380,3=http://192.168.0.86:2380,4=http://192.168.0.124:2380,5=http://192.168.0.154:2380 --initial-cluster-state=existing --pre-vote=false --log-level=panic > etcd.5.out 2>&1 &"}
	var clusters =[]string{"192.168.0.32:2380", "192.168.0.40:2380", "192.168.0.86:2380"}
	for i, new_member := range member_list {
		log.Printf("starting")
		fmt.Print(i)
		log.Printf(new_member)
		// get members' id and find the leader before add
		leaderId := uint64(0)
		leaderEp := ""
		leaderClrIdx := -1
		for idx, clr := range clusters {
			cli := mustCreateClient(clr)
			log.Printf(clr)
			resp, err := cli.Status(context.TODO(), clr)
			if err != nil {
				panic(fmt.Sprintf("get status for endpoint %v failed: %v", clr, err.Error()))
			}
			if leaderId != 0 && leaderId != resp.Leader {
				panic(fmt.Sprintf("leader not same: %v and %v", leaderId, resp.Leader))
			}
			leaderId = resp.Leader
			if resp.Header.MemberId == leaderId {
				leaderEp = clr
				leaderClrIdx = idx
			}
			if err = cli.Close(); err != nil {
				panic(err)
			}
			
		}
		log.Printf("here")
		clusters= append(clusters, new_member+":2380")
		if leaderEp == "" || leaderClrIdx == -1 {
			panic("leader not found")
		} else {
			log.Printf("found leader %v at endpoint %v\n", leaderId, leaderEp)
		}

		stopCh := make(chan struct{})

		addDoneCh := make(chan struct{})

		// add memeber
		log.Printf("ready to start: ")
		log.Print(i)
		addCli := mustCreateClient(leaderEp)
		
		//log.Print(<-time.After(time.Duration(cfg.Before) * time.Second))

		// issue add
		
		
		start := time.Now()
		ctx, _ := context.WithTimeout(context.Background(), time.Minute*5)
		//issue := time.Now()
		var nm = []string{"http://"+new_member+":2380"}
		if _, err := addCli.MemberAdd(ctx, nm, 0); err != nil {
			panic(fmt.Sprintf("add failed: %v", err))
		}
		log.Print(time.Since(start))
		close(addDoneCh)
		close(stopCh)
		addCli.Close()

		host := new_member+":22"
		
		user := "jk"
	
		var err error
		var signer ssh.Signer

		signer, err = ssh.ParsePrivateKey(pKey)
		if err != nil {
			fmt.Println(err.Error())
		}

		var hostkeyCallback ssh.HostKeyCallback
		hostkeyCallback, err = knownhosts.New("/home/ubuntu/.ssh/known_hosts")
		if err != nil {
			fmt.Println(err.Error())
		}

		conf := &ssh.ClientConfig{
			User:            user,
			HostKeyCallback: hostkeyCallback,
			Auth: []ssh.AuthMethod{
				ssh.Password("joshuakang"),
				ssh.PublicKeys(signer),
			},
		}

		var conn *ssh.Client

		conn, err = ssh.Dial("tcp", host, conf)
		if err != nil {
			fmt.Println(err.Error())
		}
		 
		session, err := conn.NewSession()
		if err != nil {
			fmt.Print(err)
		}

		
		err = session.Run(cmd_list[i])
		session.Close()
		conn.Close()
		
		time.Sleep(30*time.Second)
		log.Printf("done")
		fmt.Print(i)
	}
}
func getAddMemberList(clusters [][]uint64) []string {
	//var clrs = []string{"http://192.168.0.101:2380", "http://192.168.0.99:2380", "http://192.168.0.65:2380", "http://192.168.0.181:2380", "http://192.168.0.81:2380"}
	var clrs = []string{"http://192.168.0.101:2380"}
	//var clrs = []string{"http://192.168.0.99:2380"}
	for _, clr := range clusters {
		mems := make([]etcdserverpb.Member, 0)
		for _, id := range clr {
			mems = append(mems, etcdserverpb.Member{ID: id})
		}
		//clrs = append(clrs, etcdserverpb.MemberList{Members: mems})
	}
	return clrs
}

func getAddMeasure(ep string) addMeasure {
	cli := mustCreateClient(ep)
	defer cli.Close()

	resp, err := cli.Get(context.TODO(), "measurement")
	if err != nil {
		panic(fmt.Sprintf("fetch measurment from endpoint %v failed: %v", ep, err))
	}
	if len(resp.Kvs) != 1 {
		panic(fmt.Sprintf("invalidate measurement fetched: %v", resp.Kvs))
	}

	var m addMeasure
	if err = json.Unmarshal(resp.Kvs[0].Value, &m); err != nil {
		panic(fmt.Sprintf("unmarshall measurement failed: %v", err))
	}
	return m
}
