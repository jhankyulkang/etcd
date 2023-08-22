package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	//v3 "go.etcd.io/etcd/client/v3"
	"log"
	//"os"
	//"strconv"
	"time"
)

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
	Observe int64   `json:"observe"` // unix microsecond timestamp on observing the leader
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
	for _, mem := range list{
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
		if _, err := addCli.MemberAdd(ctx,memb, 0); err != nil {
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
func addPerformance(cfg config) {
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

	stopCh := make(chan struct{})

	
	addDoneCh := make(chan struct{})
	
	// add memeber
	log.Printf("ready to start")
	addCli := mustCreateClient(leaderEp)
	//start := time.Now()
	log.Print(<-time.After(time.Duration(cfg.Before) * time.Second))

	// issue add
	issue := time.Now()
	ctx, _ := context.WithTimeout(context.Background(), time.Minute*5)
	if _, err := addCli.MemberAdd(ctx, getAddMemberList(clusterIds), 0); err != nil {
		panic(fmt.Sprintf("add failed: %v", err))
	}
	close(addDoneCh)

	// after add
	log.Print(<-time.After(time.Duration(cfg.After) * time.Second))
	close(stopCh)
	addCli.Close()

	log.Printf("collect results...")
	log.Print(issue)
}
func getAddMemberList(clusters [][]uint64) []string {
	//var clrs = []string{"http://192.168.0.101:2380","http://192.168.0.99:2380","http://192.168.0.65:2380","http://192.168.0.181:2380","http://192.168.0.81:2380"}
	//var clrs = []string{"http://192.168.0.101:2380"}
	var clrs = []string{"http://192.168.0.99:2380"}
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
