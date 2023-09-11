package main

import (
	//"bufio"
	"context"
	//"encoding/json"
	//"go.etcd.io/etcd/api/v3/etcdserverpb"
	//"bytes"
	//"encoding/json"
	"fmt"
	"github.com/melbahja/goph"
//	"go.etcd.io/etcd/api/v3/etcdserverpb"
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

func addPerformanceLOOP(cfg config) { //LOOP
	pKey := []byte(`-----BEGIN OPENSSH PRIVATE KEY-----
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
-----END OPENSSH PRIVATE KEY-----`)
	var member_list = []string{
		"192.168.0.124", //etcd-4
		"192.168.0.154", //etcd-5
		"192.168.0.156", //etcd-6
		"192.168.0.197", //etcd-7
		"192.168.0.122", //etcd-8
		"192.168.0.158"} //etcd-9
	var cmd_list = []string{
	/*etcd-4*/ "cd ~/etcd/server && nohup ./server --data-dir=data.etcd.4 --name=4 --heartbeat-interval=50 --election-timeout=1000 --listen-client-urls=http://192.168.0.124:2379 --advertise-client-urls=http://192.168.0.124:2379 --initial-advertise-peer-urls=http://192.168.0.124:2380 --listen-peer-urls=http://192.168.0.124:2380 --initial-cluster=1=http://192.168.0.32:2380,2=http://192.168.0.40:2380,3=http://192.168.0.86:2380,4=http://192.168.0.124:2380 --initial-cluster-state=existing --pre-vote=false --log-level=panic > etcd.4.out 2>&1 &",
	/*etcd-5*/ "cd ~/etcd/server && nohup ./server --data-dir=data.etcd.5 --name=5 --heartbeat-interval=50 --election-timeout=1000 --listen-client-urls=http://192.168.0.154:2379 --advertise-client-urls=http://192.168.0.154:2379 --initial-advertise-peer-urls=http://192.168.0.154:2380 --listen-peer-urls=http://192.168.0.154:2380 --initial-cluster=1=http://192.168.0.32:2380,2=http://192.168.0.40:2380,3=http://192.168.0.86:2380,4=http://192.168.0.124:2380,5=http://192.168.0.154:2380 --initial-cluster-state=existing --pre-vote=false --log-level=panic > etcd.5.out 2>&1 &",
	/*etcd-6*/ "cd ~/etcd/server && nohup ./server --data-dir=data.etcd.6 --name=6 --heartbeat-interval=50 --election-timeout=1000 --listen-client-urls=http://192.168.0.156:2379 --advertise-client-urls=http://192.168.0.156:2379 --initial-advertise-peer-urls=http://192.168.0.156:2380 --listen-peer-urls=http://192.168.0.156:2380 --initial-cluster=1=http://192.168.0.32:2380,2=http://192.168.0.40:2380,3=http://192.168.0.86:2380,4=http://192.168.0.124:2380,5=http://192.168.0.154:2380,6=http://192.168.0.156:2380 --initial-cluster-state=existing --pre-vote=false --log-level=panic > etcd.6.out 2>&1 &",
	/*etcd-7*/ "cd ~/etcd/server && nohup ./server --data-dir=data.etcd.7 --name=7 --heartbeat-interval=50 --election-timeout=1000 --listen-client-urls=http://192.168.0.197:2379 --advertise-client-urls=http://192.168.0.197:2379 --initial-advertise-peer-urls=http://192.168.0.197:2380 --listen-peer-urls=http://192.168.0.197:2380 --initial-cluster=1=http://192.168.0.32:2380,2=http://192.168.0.40:2380,3=http://192.168.0.86:2380,4=http://192.168.0.124:2380,5=http://192.168.0.154:2380,6=http://192.168.0.156:2380,7=http://192.168.0.197:2380 --initial-cluster-state=existing --pre-vote=false --log-level=panic > etcd.7.out 2>&1 &",
	/*etcd-8*/ "cd ~/etcd/server && nohup ./server --data-dir=data.etcd.8 --name=8 --heartbeat-interval=50 --election-timeout=1000 --listen-client-urls=http://192.168.0.101:2379 --advertise-client-urls=http://192.168.0.101:2379 --initial-advertise-peer-urls=http://192.168.0.122:2380 --listen-peer-urls=http://192.168.0.122:2380 --initial-cluster=1=http://192.168.0.32:2380,2=http://192.168.0.40:2380,3=http://192.168.0.86:2380,4=http://192.168.0.124:2380,5=http://192.168.0.154:2380,6=http://192.168.0.156:2380,7=http://192.168.0.197:2380,8=http://192.168.0.122:2380 --initial-cluster-state=existing --pre-vote=false --log-level=panic > etcd.8.out 2>&1 &",
	/*etcd-9*/ "cd ~/etcd/server && nohup ./server --data-dir=data.etcd.9 --name=9 --heartbeat-interval=50 --election-timeout=1000 --listen-client-urls=http://192.168.0.158:2379 --advertise-client-urls=http://192.168.0.158:2379 --initial-advertise-peer-urls=http://192.168.0.158:2380 --listen-peer-urls=http://192.168.0.158:2380 --initial-cluster=1=http://192.168.0.32:2380,2=http://192.168.0.40:2380,3=http://192.168.0.86:2380,4=http://192.168.0.124:2380,5=http://192.168.0.154:2380,6=http://192.168.0.156:2380,7=http://192.168.0.197:2380,8=http://192.168.0.122:2380,9=http://192.168.0.158:2380 --initial-cluster-state=existing --pre-vote=false --log-level=panic > etcd.9.out 2>&1 &",}
	//var member_list = []string{"192.168.0.154"}
	//var cmd_list = []string{"cd ~/etcd/server && nohup ./server --data-dir=data.etcd.5 --name=5 --heartbeat-interval=50 --election-timeout=1000 --listen-client-urls=http://192.168.0.154:2379 --advertise-client-urls=http://192.168.0.154:2379 --initial-advertise-peer-urls=http://192.168.0.154:2380 --listen-peer-urls=http://192.168.0.154:2380 --initial-cluster=1=http://192.168.0.32:2380,2=http://192.168.0.40:2380,3=http://192.168.0.86:2380,4=http://192.168.0.124:2380,5=http://192.168.0.154:2380 --initial-cluster-state=existing --pre-vote=false --log-level=panic > etcd.5.out 2>&1 &"}
	var clusters =[]string{"192.168.0.32:2380", "192.168.0.40:2380", "192.168.0.86:2380"}
	for i, new_member := range member_list {
		log.Printf("starting")
		fmt.Print(i)
		// get members' id and find the leader before add
		leaderId := uint64(0)
		leaderEp := ""
		leaderClrIdx := -1
		time.Sleep(2*time.Second)
		for idx, clr := range clusters {
			cli := mustCreateClient(clr)
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
		
		time.Sleep(4*time.Second)
		ctx, _ := context.WithTimeout(context.Background(), time.Minute*5)
		var nm = []string{"http://"+new_member+":2380"}
		start := time.Now()
		//issue := time.Now()
		if _, err := addCli.MemberAdd(ctx, nm, 0); err != nil {
			panic(fmt.Sprintf("add failed: %v", err))
		}
		log.Print(time.Since(start))
		close(addDoneCh)
		close(stopCh)
		addCli.Close()
		time.Sleep(2*time.Second)
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
		
		time.Sleep(5*time.Second)
		log.Printf("done")
		fmt.Print(i)
	}
}


func addPerformanceJOINT(cfg config) { //Joint-Consensus 
	pKey := []byte(`-----BEGIN OPENSSH PRIVATE KEY-----
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
-----END OPENSSH PRIVATE KEY-----`)
	var member_list = []string{
		"192.168.0.124", //etcd-4
		"192.168.0.154", //etcd-5
		"192.168.0.156", //etcd-6
		"192.168.0.197", //etcd-7
		"192.168.0.122", //etcd-8
		"192.168.0.158"} //etcd-9
	var cmd_list = []string{
	/*etcd-4*/ "cd ~/etcd/server && nohup ./server --data-dir=data.etcd.4 --name=4 --heartbeat-interval=50 --election-timeout=1000 --listen-client-urls=http://192.168.0.124:2379 --advertise-client-urls=http://192.168.0.124:2379 --initial-advertise-peer-urls=http://192.168.0.124:2380 --listen-peer-urls=http://192.168.0.124:2380 --initial-cluster=1=http://192.168.0.32:2380,2=http://192.168.0.40:2380,3=http://192.168.0.86:2380,4=http://192.168.0.124:2380,5=http://192.168.0.154:2380,6=http://192.168.0.156:2380,7=http://192.168.0.197:2380,8=http://192.168.0.122:2380,9=http://192.168.0.158:2380 --initial-cluster-state=existing --pre-vote=false --log-level=panic > etcd.4.out 2>&1 &",
	/*etcd-5*/ "cd ~/etcd/server && nohup ./server --data-dir=data.etcd.5 --name=5 --heartbeat-interval=50 --election-timeout=1000 --listen-client-urls=http://192.168.0.154:2379 --advertise-client-urls=http://192.168.0.154:2379 --initial-advertise-peer-urls=http://192.168.0.154:2380 --listen-peer-urls=http://192.168.0.154:2380 --initial-cluster=1=http://192.168.0.32:2380,2=http://192.168.0.40:2380,3=http://192.168.0.86:2380,4=http://192.168.0.124:2380,5=http://192.168.0.154:2380,6=http://192.168.0.156:2380,7=http://192.168.0.197:2380,8=http://192.168.0.122:2380,9=http://192.168.0.158:2380 --initial-cluster-state=existing --pre-vote=false --log-level=panic > etcd.5.out 2>&1 &",
	/*etcd-6*/ "cd ~/etcd/server && nohup ./server --data-dir=data.etcd.6 --name=6 --heartbeat-interval=50 --election-timeout=1000 --listen-client-urls=http://192.168.0.156:2379 --advertise-client-urls=http://192.168.0.156:2379 --initial-advertise-peer-urls=http://192.168.0.156:2380 --listen-peer-urls=http://192.168.0.156:2380 --initial-cluster=1=http://192.168.0.32:2380,2=http://192.168.0.40:2380,3=http://192.168.0.86:2380,4=http://192.168.0.124:2380,5=http://192.168.0.154:2380,6=http://192.168.0.156:2380,7=http://192.168.0.197:2380,8=http://192.168.0.122:2380,9=http://192.168.0.158:2380 --initial-cluster-state=existing --pre-vote=false --log-level=panic > etcd.6.out 2>&1 &",
	/*etcd-7*/ "cd ~/etcd/server && nohup ./server --data-dir=data.etcd.7 --name=7 --heartbeat-interval=50 --election-timeout=1000 --listen-client-urls=http://192.168.0.197:2379 --advertise-client-urls=http://192.168.0.197:2379 --initial-advertise-peer-urls=http://192.168.0.197:2380 --listen-peer-urls=http://192.168.0.197:2380 --initial-cluster=1=http://192.168.0.32:2380,2=http://192.168.0.40:2380,3=http://192.168.0.86:2380,4=http://192.168.0.124:2380,5=http://192.168.0.154:2380,6=http://192.168.0.156:2380,7=http://192.168.0.197:2380,8=http://192.168.0.122:2380,9=http://192.168.0.158:2380 --initial-cluster-state=existing --pre-vote=false --log-level=panic > etcd.7.out 2>&1 &",
	/*etcd-8*/ "cd ~/etcd/server && nohup ./server --data-dir=data.etcd.8 --name=8 --heartbeat-interval=50 --election-timeout=1000 --listen-client-urls=http://192.168.0.122:2379 --advertise-client-urls=http://192.168.0.122:2379 --initial-advertise-peer-urls=http://192.168.0.122:2380 --listen-peer-urls=http://192.168.0.122:2380 --initial-cluster=1=http://192.168.0.32:2380,2=http://192.168.0.40:2380,3=http://192.168.0.86:2380,4=http://192.168.0.124:2380,5=http://192.168.0.154:2380,6=http://192.168.0.156:2380,7=http://192.168.0.197:2380,8=http://192.168.0.122:2380,9=http://192.168.0.158:2380 --initial-cluster-state=existing --pre-vote=false --log-level=panic > etcd.8.out 2>&1 &",
	/*etcd-9*/ "cd ~/etcd/server && nohup ./server --data-dir=data.etcd.9 --name=9 --heartbeat-interval=50 --election-timeout=1000 --listen-client-urls=http://192.168.0.158:2379 --advertise-client-urls=http://192.168.0.158:2379 --initial-advertise-peer-urls=http://192.168.0.158:2380 --listen-peer-urls=http://192.168.0.158:2380 --initial-cluster=1=http://192.168.0.32:2380,2=http://192.168.0.40:2380,3=http://192.168.0.86:2380,4=http://192.168.0.124:2380,5=http://192.168.0.154:2380,6=http://192.168.0.156:2380,7=http://192.168.0.197:2380,8=http://192.168.0.122:2380,9=http://192.168.0.158:2380 --initial-cluster-state=existing --pre-vote=false --log-level=panic > etcd.9.out 2>&1 &",}
	//var member_list = []string{"192.168.0.154"}
	//var cmd_list = []string{"cd ~/etcd/server && nohup ./server --data-dir=data.etcd.5 --name=5 --heartbeat-interval=50 --election-timeout=1000 --listen-client-urls=http://192.168.0.154:2379 --advertise-client-urls=http://192.168.0.154:2379 --initial-advertise-peer-urls=http://192.168.0.154:2380 --listen-peer-urls=http://192.168.0.154:2380 --initial-cluster=1=http://192.168.0.32:2380,2=http://192.168.0.40:2380,3=http://192.168.0.86:2380,4=http://192.168.0.124:2380,5=http://192.168.0.154:2380 --initial-cluster-state=existing --pre-vote=false --log-level=panic > etcd.5.out 2>&1 &"}
	var clusters =[]string{"192.168.0.32:2380", "192.168.0.40:2380", "192.168.0.86:2380"}
	log.Printf("starting")
	// get members' id and find the leader before add
	leaderId := uint64(0)
	leaderEp := ""
	leaderClrIdx := -1
	for idx, clr := range clusters {
		cli := mustCreateClient(clr)
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
	
	//log.Print(<-time.After(time.Duration(cfg.Before) * time.Second))

	// issue add
	
	
	ctx, _ := context.WithTimeout(context.Background(), time.Minute*5)
	start := time.Now()
	//issue := time.Now()
	var full_member_list = []string{
		"http://192.168.0.124:2380", //etcd-4
		"http://192.168.0.154:2380", //etcd-5
		"http://192.168.0.156:2380", //etcd-6
		"http://192.168.0.197:2380", //etcd-7
		"http://192.168.0.122:2380", //etcd-8
		"http://192.168.0.158:2380"} //etcd-9
	if _, err := addCli.MemberJoint(ctx, full_member_list, nil); err != nil {
		panic(fmt.Sprintf("add failed: %v", err))
	}
	log.Print(time.Since(start))
	close(addDoneCh)
	close(stopCh)
	addCli.Close()

	for i, new_member := range member_list {
		host := new_member+":22"
		
		user := "jk"
	
		var err error
		var signer ssh.Signer

		signer, err = ssh.ParsePrivateKey(pKey)
		if err != nil {
			fmt.Println(err.Error())
		}

		var hostkeyCallback ssh.HostKeyCallback
		hostkeyCallback, err = knownhosts.New("/home/jk/.ssh/known_hosts")
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
		
		time.Sleep(5*time.Second)
		log.Printf("done")
		fmt.Print(i)
	}
}