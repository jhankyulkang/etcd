// Copyright 2016 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clientv3

import (
	"context"
	"fmt"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/pkg/v3/types"

	"google.golang.org/grpc"
)

type (
	Member                etcdserverpb.Member
	MemberListResponse    etcdserverpb.MemberListResponse
	MemberAddResponse     etcdserverpb.MemberAddResponse
	MemberRemoveResponse  etcdserverpb.MemberRemoveResponse
	MemberUpdateResponse  etcdserverpb.MemberUpdateResponse
	MemberPromoteResponse etcdserverpb.MemberPromoteResponse
	MemberSplitResponse   etcdserverpb.MemberSplitResponse
	MemberMergeResponse   etcdserverpb.MemberMergeResponse
	MemberJointResponse   etcdserverpb.MemberJointResponse
)

type Cluster interface {
	// MemberList lists the current cluster membership.
	MemberList(ctx context.Context) (*MemberListResponse, error)

	// MemberAdd adds a new member into the cluster.
	MemberAdd(ctx context.Context, peerAddrs []string, quorum uint64) (*MemberAddResponse, error)
	// MemberAddAsLearner adds a new learner member into the cluster.
	MemberAddAsLearner(ctx context.Context, peerAddrs []string, quorum uint64) (*MemberAddResponse, error)

	// MemberRemove removes an existing member from the cluster.
	MemberRemove(ctx context.Context, id uint64) (*MemberRemoveResponse, error)

	// MemberUpdate updates the peer addresses of the member.
	MemberUpdate(ctx context.Context, id uint64, peerAddrs []string) (*MemberUpdateResponse, error)

	// MemberPromote promotes a member from raft learner (non-voting) to raft voting member.
	MemberPromote(ctx context.Context, id uint64) (*MemberPromoteResponse, error)

	MemberSplit(ctx context.Context, clusters []etcdserverpb.MemberList, explictLeave, leave bool) (*MemberSplitResponse, error)

	MemberMerge(ctx context.Context, clusters map[uint64]etcdserverpb.MemberList) (*MemberMergeResponse, error)

	MemberJoint(ctx context.Context, addPeersAddr []string, removePeersId []uint64) (*MemberJointResponse, error)
}

type cluster struct {
	remote   etcdserverpb.ClusterClient
	callOpts []grpc.CallOption
}

func NewCluster(c *Client) Cluster {
	api := &cluster{remote: RetryClusterClient(c)}
	if c != nil {
		api.callOpts = c.callOpts
	}
	return api
}

func NewClusterFromClusterClient(remote etcdserverpb.ClusterClient, c *Client) Cluster {
	api := &cluster{remote: remote}
	if c != nil {
		api.callOpts = c.callOpts
	}
	return api
}

func (c *cluster) MemberAdd(ctx context.Context, peerAddrs []string, quorum uint64) (*MemberAddResponse, error) {
	return c.memberAdd(ctx, peerAddrs, false, quorum)
}

func (c *cluster) MemberAddAsLearner(ctx context.Context, peerAddrs []string, quorum uint64) (*MemberAddResponse, error) {
	return c.memberAdd(ctx, peerAddrs, true, quorum)
}

// changed by Joshua to include quorum value
func (c *cluster) memberAdd(ctx context.Context, peerAddrs []string, isLearner bool, quorum uint64) (*MemberAddResponse, error) {
	// fail-fast before panic in rafthttp
	if _, err := types.NewURLs(peerAddrs); err != nil {
		return nil, err
	}
	//panic(quorum)
	var r *etcdserverpb.MemberAddRequest
	if quorum == 0 {
		r = &etcdserverpb.MemberAddRequest{
			PeerURLs:  peerAddrs,
			IsLearner: isLearner,
		}
	} else {
		r = &etcdserverpb.MemberAddRequest{
			PeerURLs:  peerAddrs,
			IsLearner: isLearner,
			Quorum:    quorum,
		}
	}

	resp, err := c.remote.MemberAdd(ctx, r, c.callOpts...)
	if err != nil {
		fmt.Print("error")
		return nil, toErr(ctx, err)
	}
	return (*MemberAddResponse)(resp), nil
}

func (c *cluster) MemberRemove(ctx context.Context, id uint64) (*MemberRemoveResponse, error) {
	r := &etcdserverpb.MemberRemoveRequest{ID: id}
	resp, err := c.remote.MemberRemove(ctx, r, c.callOpts...)
	if err != nil {
		return nil, toErr(ctx, err)
	}
	return (*MemberRemoveResponse)(resp), nil
}

func (c *cluster) MemberUpdate(ctx context.Context, id uint64, peerAddrs []string) (*MemberUpdateResponse, error) {
	// fail-fast before panic in rafthttp
	if _, err := types.NewURLs(peerAddrs); err != nil {
		return nil, err
	}

	// it is safe to retry on update.
	r := &etcdserverpb.MemberUpdateRequest{ID: id, PeerURLs: peerAddrs}
	resp, err := c.remote.MemberUpdate(ctx, r, c.callOpts...)
	if err == nil {
		return (*MemberUpdateResponse)(resp), nil
	}
	return nil, toErr(ctx, err)
}

func (c *cluster) MemberList(ctx context.Context) (*MemberListResponse, error) {
	// it is safe to retry on list.
	resp, err := c.remote.MemberList(ctx, &etcdserverpb.MemberListRequest{Linearizable: true}, c.callOpts...)
	if err == nil {
		return (*MemberListResponse)(resp), nil
	}
	return nil, toErr(ctx, err)
}

func (c *cluster) MemberPromote(ctx context.Context, id uint64) (*MemberPromoteResponse, error) {
	r := &etcdserverpb.MemberPromoteRequest{ID: id}
	resp, err := c.remote.MemberPromote(ctx, r, c.callOpts...)
	if err != nil {
		return nil, toErr(ctx, err)
	}
	return (*MemberPromoteResponse)(resp), nil
}

func (c *cluster) MemberSplit(ctx context.Context, clusters []etcdserverpb.MemberList, explictLeave, leave bool) (*MemberSplitResponse, error) {
	r := &etcdserverpb.MemberSplitRequest{Clusters: clusters, ExplicitLeave: explictLeave, Leave: leave}
	resp, err := c.remote.MemberSplit(ctx, r, c.callOpts...)
	if err != nil {
		return nil, toErr(ctx, err)
	}
	return (*MemberSplitResponse)(resp), nil
}

func (c *cluster) MemberMerge(ctx context.Context, clusters map[uint64]etcdserverpb.MemberList) (*MemberMergeResponse, error) {
	r := &etcdserverpb.MemberMergeRequest{Clusters: clusters}
	resp, err := c.remote.MemberMerge(ctx, r, c.callOpts...)
	if err != nil {
		return nil, toErr(ctx, err)
	}
	return (*MemberMergeResponse)(resp), nil
}

func (c *cluster) MemberJoint(ctx context.Context, addPeersAddr []string, removePeersId []uint64) (*MemberJointResponse, error) {
	r := &etcdserverpb.MemberJointRequest{AddPeersUrl: addPeersAddr, RemovePeersId: removePeersId}
	resp, err := c.remote.MemberJoint(ctx, r, c.callOpts...)
	if err != nil {
		return nil, toErr(ctx, err)
	}
	return (*MemberJointResponse)(resp), nil
}
