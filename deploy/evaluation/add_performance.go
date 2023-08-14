package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	v3 "go.etcd.io/etcd/client/v3"
	"log"
	"os"
	"strconv"
	"time"
)

type addReport struct {
	Start int64 `json:"start"`
	Issue int64 `json:"issue"`

	Queries []query `json:"queries"` // queries send to original leader cluster

	Observes []observe `json:"observes"` // observe on non-original-leader cluster

	Leader   splitMeasure   `json:"leader"`   // measure collected from origin leader
	Measures []splitMeasure `json:"measures"` // measures collected from non-original-leader cluster
}

type splitMeasure struct {
	SplitEnter  int64 `json:"splitEnter"`
	SplitLeave  int64 `json:"splitLeave"`
	LeaderElect int64 `json:"leaderElect"`
}

type observe struct {
	Observe int64   `json:"observe"` // unix microsecond timestamp on observing the leader
	Queries []query `json:"queries"`
}

type query struct {
	Start   int64 `json:"start"`   // unix microsecond timestamp
	Latency int64 `json:"latency"` // in microsecond
}

func splitPerformance(cfg config) {
	// get members' id and find the leader before add
	clusterIds := make([][]uint64, len(cfg.Clusters))
	leaderId := uint64(0)
	leaderEp := ""
	leaderClrIdx := -1
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
	//run to here

	// spawn thread to query on leader
	log.Printf("spawn requesters...")
	startQuery := make(chan struct{})
	queryCh := make(chan []query) // one []query for one requester
	splitDoneCh := make(chan struct{})
	cli := mustCreateClient(leaderEp)
	defer cli.Close()
	for i := 0; i < int(cfg.Threads)*len(cfg.Clusters); i++ {
		go func(tidx int) {
			<-startQuery
			queries := make([]query, 0)
			for qidx := 0; ; qidx++ {
				if tidx >= int(cfg.Threads) {
					select {
					case <-splitDoneCh:
						queryCh <- queries
						return
					default:
						break
					}
				}
				select {
				default:
					s := time.Now()
					ctx, _ := context.WithTimeout(context.TODO(), time.Minute*5)
					if _, err := cli.Do(ctx, v3.OpPut(fmt.Sprintf("thread-%v-%v", tidx, qidx), strconv.Itoa(qidx))); err != nil {
						log.Printf("thread %v sending request #%v error: %v", tidx, qidx, err)
						continue
					}
					queries = append(queries, query{s.UnixMicro(), time.Since(s).Microseconds()})
				case <-stopCh:
					queryCh <- queries
					return
				}
			}
		}(i)
	}

	// spawn thread to observe non-leader clusters and send queries after leave
	log.Printf("spawn observers...")
	observeCh := make(chan observe) // one observeCh to collect queries from all non-original-leader clusters
	for idx, clr := range cfg.Clusters {
		if idx == leaderClrIdx {
			continue
		}
		for _, ep := range clr {
			go func(ep string, oldLeader uint64, clrIdx int, ids []uint64) {
				cli := mustCreateClient(ep)
				defer cli.Close()

				<-splitDoneCh

				// check if this endpoint is leader
				obTime := time.Now()
				for {
					resp, err := cli.Status(context.TODO(), ep)
					if err != nil {
						log.Printf("observe %v error: %v", ep, err)
					}
					if resp.Leader != 0 && resp.Leader != oldLeader {
						found := false
						for _, id := range ids {
							if id == resp.Leader {
								found = true
								break
							}
						}
						if found {
							if resp.Leader == resp.Header.MemberId {
								obTime = time.Now()
								break // if this client connects to leader, start sending queries
							} else {
								cli.Close()
								return // if not leader, return
							}
						}
					}
				}

				observeQueryCh := make(chan []query)
				for i := 0; i < int(cfg.Threads); i++ {
					go func(tidx int, ep string) {
						queries := make([]query, 0)
						for qidx := 0; ; qidx++ {
							select {
							default:
								s := time.Now()
								ctx, _ := context.WithTimeout(context.Background(), time.Minute*5)
								if _, err := cli.Do(ctx, v3.OpPut(fmt.Sprintf("observer-%v-%v", tidx, qidx), strconv.Itoa(qidx))); err != nil {
									log.Printf("observer %v-%v sending request #%v error: %v", clrIdx, tidx, qidx, err)
									continue
								}
								queries = append(queries, query{s.UnixMicro(), time.Since(s).Microseconds()})
							case <-stopCh:
								observeQueryCh <- queries
								return
							}
						}
					}(i, ep)
				}

				queries := make([]query, 0)
				for i := 0; i < int(cfg.Threads); i++ {
					queries = append(queries, <-observeQueryCh...)
				}
				observeCh <- observe{Observe: obTime.UnixMicro(), Queries: queries}
			}(ep, leaderId, idx, clusterIds[idx])
		}
	}

	// before split
	log.Printf("ready to start")
	splitCli := mustCreateClient(leaderEp)
	start := time.Now()
	close(startQuery)
	<-time.After(time.Duration(cfg.Before) * time.Second)

	// issue split
	issue := time.Now()
	ctx, _ := context.WithTimeout(context.Background(), time.Minute*5)
	if _, err := splitCli.MemberSplit(ctx, getSplitMemberList(clusterIds), false, false); err != nil {
		panic(fmt.Sprintf("split failed: %v", err))
	}
	close(splitDoneCh)

	// after split
	<-time.After(time.Duration(cfg.After) * time.Second)
	close(stopCh)
	splitCli.Close()

	log.Printf("collect results...")

	// fetch queries
	queries := make([]query, 0)
	for i := 0; i < int(cfg.Threads)*len(cfg.Clusters); i++ {
		qs := <-queryCh
		log.Printf("requester fetch %v queries", len(qs))
		queries = append(queries, qs...)
	}

	// fetch observations
	observes := make([]observe, 0)
	for i := 0; i < len(cfg.Clusters)-1; i++ {
		ob := <-observeCh
		log.Printf("observer start at %v fetch %v queries", ob.Observe/1e6, len(ob.Queries))
		observes = append(observes, ob)
	}

	// fetch split measurement from server
	var leaderMeasure splitMeasure
	measures := make([]splitMeasure, 0)
	for idx, clr := range cfg.Clusters {
		if idx == leaderClrIdx {
			for _, ep := range clr {
				if ep == leaderEp {
					leaderMeasure = getSplitMeasure(ep)
					log.Printf("leader measure: %v, %v, %v",
						leaderMeasure.SplitEnter, leaderMeasure.SplitLeave, leaderMeasure.LeaderElect)
				}
			}
		}
		for _, ep := range clr {
			m := getSplitMeasure(ep)
			log.Printf("measure: %v, %v, %v", m.SplitEnter, m.SplitLeave, m.LeaderElect)
			measures = append(measures, m)
		}
	}

	// write report to file
	data, err := json.Marshal(splitReport{
		Start:    start.UnixMicro(),
		Issue:    issue.UnixMicro(),
		Leader:   leaderMeasure,
		Queries:  queries,
		Observes: observes,
		Measures: measures})
	if err != nil {
		panic(fmt.Sprintf("marshal split report failed: %v", err))
	}
	if err = os.WriteFile(fmt.Sprintf("%v/split-%v-%v.json", cfg.Folder, len(cfg.Clusters), cfg.Threads),
		data, 0666); err != nil {
		panic(fmt.Sprintf("write report json failed: %v", err))
	}

	log.Printf("finished.")
}

func getSplitMemberList(clusters [][]uint64) []etcdserverpb.MemberList {
	clrs := make([]etcdserverpb.MemberList, 0)
	for _, clr := range clusters {
		mems := make([]etcdserverpb.Member, 0)
		for _, id := range clr {
			mems = append(mems, etcdserverpb.Member{ID: id})
		}
		clrs = append(clrs, etcdserverpb.MemberList{Members: mems})
	}
	return clrs
}

func getSplitMeasure(ep string) splitMeasure {
	cli := mustCreateClient(ep)
	defer cli.Close()

	resp, err := cli.Get(context.TODO(), "measurement")
	if err != nil {
		panic(fmt.Sprintf("fetch measurment from endpoint %v failed: %v", ep, err))
	}
	if len(resp.Kvs) != 1 {
		panic(fmt.Sprintf("invalidate measurement fetched: %v", resp.Kvs))
	}

	var m splitMeasure
	if err = json.Unmarshal(resp.Kvs[0].Value, &m); err != nil {
		panic(fmt.Sprintf("unmarshall measurement failed: %v", err))
	}
	return m
}
