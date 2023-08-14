package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"log"
	"os"
	"strconv"
	"time"
)

type mergeReport struct {
	Start    int64          `json:"start"`
	Issue    int64          `json:"issue"`
	Complete int64          `json:"complete"`
	Measures []mergeMeasure `json:"measures"`
	Queries  [][]query      `json:"queries"` // queries before merge
	Observe  observe        `json:"observe"` // observe after merge
}

type mergeMeasure struct {
	MergeTxIssue     int64 `json:"mergeTxIssue"`
	MergeTxStart     int64 `json:"mergeTxStart"`
	MergeTxCommit    int64 `json:"mergeTxCommit"`
	MergeEnter       int64 `json:"mergeEnter"`
	MergeSnapReceive int64 `json:"mergeSnapReceive"`
	MergeSnapInstall int64 `json:"mergeSnapInstall"`
	MergeLeave       int64 `json:"mergeLeave"`
	LeaderElect      int64 `json:"leaderElect"`
}

func merge(cfg config) {
	// find leader in the first cluster
	leaderEps := make([]string, 0, len(cfg.Clusters))
	for _, clr := range cfg.Clusters {
		for _, ep := range clr {
			cli := mustCreateClient(ep)
			resp, err := cli.Status(context.TODO(), ep)
			if err != nil {
				panic(fmt.Sprintf("get status for endpoint %v failed: %v", ep, err.Error()))
			}
			if err = cli.Close(); err != nil {
				panic(err)
			}

			if resp.Header.MemberId == resp.Leader {
				leaderEps = append(leaderEps, ep)
				break
			}
		}
	}
	if len(leaderEps) == 0 {
		panic("leader not found")
	} else {
		log.Printf("found leader at endpoint %v\n", leaderEps)
	}

	// get member's info of each cluster
	urls := make([]string, 0)
	for _, clr := range cfg.Clusters {
		urls = append(urls, clr[1])
		continue
	}
	clusters := getMergeMemberList(urls)

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

	stopCh := make(chan struct{})
	mergeCh := make(chan struct{})

	// spawn thread
	log.Printf("spawn threads for leaders...")
	startQuery := make(chan struct{})
	queryCh := make(chan []query)
	for cidx, leaderEp := range leaderEps {
		aggQueryCh := make(chan []query)
		go func() {
			queries := make([]query, 0)
			for i := 0; i < int(cfg.Threads); i++ {
				queries = append(queries, <-aggQueryCh...)
			}
			queryCh <- queries
		}()
		for i := 0; i < int(cfg.Threads); i++ {
			go func(cidx, tidx int, ep string) {
				cli := mustCreateClient(ep)
				defer cli.Close()

				<-startQuery
				queries := make([]query, 0)
				for qidx := 0; ; qidx++ {
					select {
					default:
						s := time.Now()
						if _, err := cli.Do(context.TODO(), clientv3.OpPut(fmt.Sprintf("thread-%v-%v-%v", cidx, tidx, qidx), strconv.Itoa(qidx))); err != nil {
							log.Printf("thread %v-%v sending request #%v error: %v", cidx, tidx, qidx, err)
							continue
						}
						queries = append(queries, query{s.UnixMicro(), time.Since(s).Microseconds()})
					case <-mergeCh:
						aggQueryCh <- queries
						return
					}
				}
			}(cidx, i, leaderEp)
		}
	}

	// spawn thread to observe leader after merge
	log.Printf("spawn observers...")
	observeCh := make(chan observe) // one observeCh to collect queries from all non-original-leader clusters
	for idx, clr := range cfg.Clusters {
		for _, ep := range clr {
			go func(ep string, clrIdx int) {
				cli := mustCreateClient(ep)
				<-mergeCh

				// check if this endpoint is leader
				for {
					sresp, err := cli.Status(context.TODO(), ep)
					if err != nil {
						log.Printf("observe status %v error: %v", ep, err)
						continue
					}
					if sresp.Leader == sresp.Header.MemberId {
						break // if this client connects to leader, start sending queries
					} else {
						cli.Close()
						return // if not leader, return
					}
				}

				obTime := time.Now()
				aggQueryCh := make(chan []query)
				for i := 0; i < int(cfg.Threads)*len(cfg.Clusters); i++ {
					go func(tidx int, ep string) {
						queries := make([]query, 0)
						for qidx := 0; ; qidx++ {
							select {
							default:
								s := time.Now()
								if _, err := cli.Do(context.TODO(), clientv3.OpPut(fmt.Sprintf("observer-%v-%v", tidx, qidx), strconv.Itoa(qidx))); err != nil {
									log.Printf("observer %v-%v sending request #%v error: %v", clrIdx, tidx, qidx, err)
									continue
								}
								queries = append(queries, query{s.UnixMicro(), time.Since(s).Microseconds()})
							case <-stopCh:
								aggQueryCh <- queries
								return
							}
						}
					}(i, ep)
				}

				aggQueries := make([]query, 0)
				for i := 0; i < int(cfg.Threads)*len(cfg.Clusters); i++ {
					aggQueries = append(aggQueries, <-aggQueryCh...)
				}
				observeCh <- observe{Observe: obTime.UnixMicro(), Queries: aggQueries}
			}(ep, idx)
		}
	}

	// before merge
	log.Printf("ready to start")
	mergeCli := mustCreateClient(leaderEps[0])
	start := time.Now()
	close(startQuery)
	<-time.After(time.Duration(cfg.Before) * time.Second)

	// issue merge
	issue := time.Now()
	if _, err := mergeCli.MemberMerge(context.TODO(), clusters); err != nil {
		panic(fmt.Sprintf("merge failed: %v", err))
	}
	complete := time.Now()
	close(mergeCh)

	// after merge
	<-time.After(time.Duration(cfg.After) * time.Second)
	close(stopCh)
	mergeCli.Close()

	log.Printf("collect results...")

	// fetch queries
	queries := make([][]query, 0)
	for i := 0; i < len(cfg.Clusters); i++ {
		qs := <-queryCh
		log.Printf("cluster-%v: %v queries", i, len(qs))
		queries = append(queries, qs)
	}

	// fetch observations
	ob := <-observeCh
	log.Printf("observer start at %v fetch %v queries", ob.Observe/1e6, len(ob.Queries))

	// fetch merge measurement from server
	log.Printf("fetch measurements...")
	measures := make([]mergeMeasure, 0)
	for _, clr := range cfg.Clusters {
		for _, ep := range clr {
			m := getMergeMeasure(ep)
			if m.MergeTxStart < 0 || m.MergeTxCommit < 0 || m.MergeEnter < 0 {
				log.Printf("invalid measure: %v", m)
			}
			measures = append(measures, m)
		}
	}

	// write report to file
	data, err := json.Marshal(mergeReport{
		Start:    start.UnixMicro(),
		Issue:    issue.UnixMicro(),
		Complete: complete.UnixMicro(),
		Queries:  queries,
		Observe:  ob,
		Measures: measures})
	if err != nil {
		panic(fmt.Sprintf("marshal merge report failed: %v", err))
	}
	if err = os.WriteFile(fmt.Sprintf("%v/merge-%v-%v-%v-%v.json", cfg.Folder, len(cfg.Clusters), cfg.Threads, cfg.Load, cfg.Repetition),
		data, 0666); err != nil {
		panic(fmt.Sprintf("write report json failed: %v", err))
	}

	log.Printf("finished.")
}

func getMergeMemberList(urls []string) map[uint64]etcdserverpb.MemberList {
	ctx := context.Background()
	clusters := map[uint64]etcdserverpb.MemberList{}
	for _, url := range urls {
		cli, err := clientv3.New(clientv3.Config{Endpoints: []string{url}})
		if err != nil {
			panic(err)
		}

		resp, err := cli.MemberList(ctx)
		if err != nil {
			panic(err)
		}
		if len(resp.Members) == 0 {
			panic(err)
		}
		if err = cli.Close(); err != nil {
			panic(err)
		}

		mems := make([]etcdserverpb.Member, 0)
		for _, mem := range resp.Members {
			mems = append(mems, *mem)
		}
		clusters[resp.Header.ClusterId] = etcdserverpb.MemberList{Members: mems}
	}

	return clusters
}

func getMergeMeasure(ep string) mergeMeasure {
	cli := mustCreateClient(ep)
	defer cli.Close()

	resp, err := cli.Get(context.TODO(), "measurement")
	if err != nil {
		panic(fmt.Sprintf("fetch measurment from endpoint %v failed: %v", ep, err))
	}
	if len(resp.Kvs) != 1 {
		panic(fmt.Sprintf("invalidate measurement fetched: %v", resp.Kvs))
	}

	var m mergeMeasure
	if err = json.Unmarshal(resp.Kvs[0].Value, &m); err != nil {
		panic(fmt.Sprintf("unmarshall measurement failed: %v", err))
	}
	return m
}
