package main

import (
	"context"
	"encoding/json"
	"fmt"
	v3 "go.etcd.io/etcd/client/v3"
	"log"
	"os"
	"strconv"
	"time"
)

func splitImpact(cfg config) {
	// get members' id and find the leader before split
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

	// spawn thread to query on leader
	log.Printf("spawn requesters...")
	startQuery := make(chan struct{})
	queryCh := make(chan []query) // one []query for one requester
	for i := 0; i < int(cfg.Threads)*len(cfg.Clusters); i++ {
		go func(tidx int) {
			cli := mustCreateClient(leaderEp)
			defer cli.Close()

			<-startQuery
			queries := make([]query, 0)
			for qidx := 0; ; qidx++ {
				select {
				default:
					s := time.Now()
					if _, err := cli.Do(context.TODO(), v3.OpPut(fmt.Sprintf("thread-%v-%v", tidx, qidx), strconv.Itoa(qidx))); err != nil {
						log.Printf("thread %v sending request #%v error: %v", tidx, qidx, err)
					}
					queries = append(queries, query{s.UnixMicro(), time.Since(s).Microseconds()})
				case <-stopCh:
					queryCh <- queries
					return
				}
			}
		}(i)
	}

	// before split
	log.Printf("ready to start")
	splitCli := mustCreateClient(leaderEp)
	start := time.Now()
	close(startQuery)
	<-time.After(time.Duration(cfg.Before) * time.Second)

	// issue split
	issue := time.Now()
	if _, err := splitCli.MemberSplit(context.TODO(), getSplitMemberList(clusterIds), false, false); err != nil {
		panic(fmt.Sprintf("split failed: %v", err))
	}

	// after split
	<-time.After(time.Duration(cfg.After) * time.Second)
	close(stopCh)
	splitCli.Close()

	log.Printf("collect results...")

	// fetch queries
	queries := make([]query, 0)
	for i := 0; i < int(cfg.Threads); i++ {
		qs := <-queryCh
		log.Printf("requester fetch %v queries", len(qs))
		queries = append(queries, qs...)
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
		Measures: measures})
	if err != nil {
		panic(fmt.Sprintf("marshal split report failed: %v", err))
	}
	if err = os.WriteFile(fmt.Sprintf("%v/split-impact-%v-%v.json", cfg.Folder, len(cfg.Clusters), cfg.Threads),
		data, 0666); err != nil {
		panic(fmt.Sprintf("write report json failed: %v", err))
	}

	log.Printf("finished.")
}
