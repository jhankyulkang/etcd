package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go.etcd.io/etcd/client/v3"
	"log"
	"os"
	"strconv"
	"time"
)

func splitLoad(cfg config) {
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

	// send requests to each cluster forehand for snapshot load
	if cfg.Load > 0 {
		log.Printf("prepare forehand load...")
		numThreads := 4
		wait := make(chan struct{})
		for cidx, clr := range cfg.Clusters {
			for i := 0; i < numThreads; i++ { // four requesters for speed
				go func(cidx int, tidx int, load int, ep []string) {
					cli, err := clientv3.New(clientv3.Config{Endpoints: ep, DialTimeout: 1 * time.Minute})
					if err != nil {
						panic(fmt.Sprintf("create client for endpoint %v failed: %v", ep, err))
					}
					defer cli.Close()

					for ridx := 0; ridx < load; ridx++ {
						if _, err := cli.Do(context.TODO(), clientv3.OpPut(fmt.Sprintf("%v-%v-%v", cidx, tidx, ridx), strconv.Itoa(ridx))); err != nil {
							log.Printf("sending to cluster %v request #%v error: %v", cidx, ridx, err)
						}
					}
					wait <- struct{}{}
					if err := cli.Close(); err != nil {
						panic(fmt.Sprintf("close client failed: %v", err))
					}
				}(cidx, i, int(cfg.Load/uint64(numThreads)), clr)
			}
		}
		for i := 0; i < numThreads*len(cfg.Clusters); i++ {
			<-wait
		}
		log.Printf("wait 10s for stable before measuring...")
		<-time.After(10 * time.Second)
	}

	// before split
	log.Printf("ready to start")
	splitCli := mustCreateClient(leaderEp)
	start := time.Now()
	<-time.After(time.Duration(cfg.Before) * time.Second)

	// issue split
	issue := time.Now()
	if _, err := splitCli.MemberSplit(context.TODO(), getSplitMemberList(clusterIds), false, false); err != nil {
		panic(fmt.Sprintf("split failed: %v", err))
	}

	// after split
	<-time.After(time.Duration(cfg.After) * time.Second)
	splitCli.Close()

	log.Printf("collect results...")

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
		Measures: measures})
	if err != nil {
		panic(fmt.Sprintf("marshal split report failed: %v", err))
	}
	if err = os.WriteFile(fmt.Sprintf("%v/split-load-%v-%v-%v.json", cfg.Folder, len(cfg.Clusters), cfg.Load, cfg.Repetition),
		data, 0666); err != nil {
		panic(fmt.Sprintf("write report json failed: %v", err))
	}

	log.Printf("finished.")
}
