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

package v3rpc

import (
	"context"
	"time"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/server/v3/etcdserver/api"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
)

type ClusterServer struct {
	cluster api.Cluster
	server  *etcdserver.EtcdServer
}

func NewClusterServer(s *etcdserver.EtcdServer) *ClusterServer {
	return &ClusterServer{
		cluster: s.Cluster(),
		server:  s,
	}
}

func (cs *ClusterServer) MemberAdd(ctx context.Context, r *etcdserverpb.MemberAddRequest) (*etcdserverpb.MemberAddResponse, error) {
	urls, err := types.NewURLs(r.PeerURLs)
	if err != nil {
		return nil, rpctypes.ErrGRPCMemberBadURLs
	}

	now := time.Now()
	var m *membership.Member
	if r.IsLearner {
		m = membership.NewMemberAsLearner("", urls, "", &now)
	} else {
		m = membership.NewMember("", urls, "", &now)
	}

	membs, merr := cs.server.AddMember(ctx, *m, r.Quorum)
	if merr != nil {
		return nil, togRPCError(merr)
	}

	return &etcdserverpb.MemberAddResponse{
		Header: cs.header(),
		Member: &etcdserverpb.Member{
			ID:        uint64(m.ID),
			PeerURLs:  m.PeerURLs,
			IsLearner: m.IsLearner,
		},
		Members: membersToProtoMembers(membs),
		//add quorum

	}, nil
}

func (cs *ClusterServer) MemberRemove(ctx context.Context, r *etcdserverpb.MemberRemoveRequest) (*etcdserverpb.MemberRemoveResponse, error) {
	membs, err := cs.server.RemoveMember(ctx, r.ID)
	if err != nil {
		return nil, togRPCError(err)
	}
	return &etcdserverpb.MemberRemoveResponse{Header: cs.header(), Members: membersToProtoMembers(membs)}, nil
}

func (cs *ClusterServer) MemberUpdate(ctx context.Context, r *etcdserverpb.MemberUpdateRequest) (*etcdserverpb.MemberUpdateResponse, error) {
	m := membership.Member{
		ID:             types.ID(r.ID),
		RaftAttributes: membership.RaftAttributes{PeerURLs: r.PeerURLs},
	}
	membs, err := cs.server.UpdateMember(ctx, m)
	if err != nil {
		return nil, togRPCError(err)
	}
	return &etcdserverpb.MemberUpdateResponse{Header: cs.header(), Members: membersToProtoMembers(membs)}, nil
}

func (cs *ClusterServer) MemberList(ctx context.Context, r *etcdserverpb.MemberListRequest) (*etcdserverpb.MemberListResponse, error) {
	if r.Linearizable {
		if err := cs.server.LinearizableReadNotify(ctx); err != nil {
			return nil, togRPCError(err)
		}
	}
	membs := membersToProtoMembers(cs.cluster.Members())
	return &etcdserverpb.MemberListResponse{Header: cs.header(), Members: membs}, nil
}

func (cs *ClusterServer) MemberPromote(ctx context.Context, r *etcdserverpb.MemberPromoteRequest) (*etcdserverpb.MemberPromoteResponse, error) {
	membs, err := cs.server.PromoteMember(ctx, r.ID)
	if err != nil {
		return nil, togRPCError(err)
	}
	return &etcdserverpb.MemberPromoteResponse{Header: cs.header(), Members: membersToProtoMembers(membs)}, nil
}

func (cs *ClusterServer) MemberSplit(ctx context.Context, r *etcdserverpb.MemberSplitRequest) (*etcdserverpb.MemberSplitResponse, error) {
	membs, err := cs.server.SplitMember(ctx, r.Clusters, r.ExplicitLeave, r.Leave)
	if err != nil {
		return nil, err
	}
	return &etcdserverpb.MemberSplitResponse{Header: cs.header(), Members: membersToProtoNonPointerMembers(membs)}, nil
}

func (cs *ClusterServer) MemberMerge(ctx context.Context, r *etcdserverpb.MemberMergeRequest) (*etcdserverpb.MemberMergeResponse, error) {
	membs, err := cs.server.MergeMember(ctx, *r)
	if err != nil {
		return nil, err
	}
	return &etcdserverpb.MemberMergeResponse{Header: cs.header(), Members: membersToProtoNonPointerMembers(membs)}, nil
}

func (cs *ClusterServer) MemberJoint(ctx context.Context, r *etcdserverpb.MemberJointRequest) (*etcdserverpb.MemberJointResponse, error) {
	addMembs := make([]membership.Member, 0)
	for _, url := range r.AddPeersUrl {
		urls, err := types.NewURLs([]string{url})
		if err != nil {
			return nil, rpctypes.ErrGRPCMemberBadURLs
		}

		now := time.Now()
		addMembs = append(addMembs, *membership.NewMember("", urls, "", &now))
	}

	membs, err := cs.server.JointMember(ctx, addMembs, r.RemovePeersId)
	if err != nil {
		return nil, err
	}
	return &etcdserverpb.MemberJointResponse{Header: cs.header(), Members: membersToProtoMembers(membs)}, nil
}

func (cs *ClusterServer) header() *etcdserverpb.ResponseHeader {
	return &etcdserverpb.ResponseHeader{ClusterId: uint64(cs.cluster.ID()), MemberId: uint64(cs.server.ID()), RaftTerm: cs.server.Term()}
}

func membersToProtoMembers(membs []*membership.Member) []*etcdserverpb.Member {
	protoMembs := make([]*etcdserverpb.Member, len(membs))
	for i := range membs {
		protoMembs[i] = &etcdserverpb.Member{
			Name:       membs[i].Name,
			ID:         uint64(membs[i].ID),
			PeerURLs:   membs[i].PeerURLs,
			ClientURLs: membs[i].ClientURLs,
			IsLearner:  membs[i].IsLearner,
		}
	}
	return protoMembs
}

func membersToProtoNonPointerMembers(membs []membership.Member) []etcdserverpb.Member {
	protoMembs := make([]etcdserverpb.Member, len(membs))
	for i := range membs {
		protoMembs[i] = etcdserverpb.Member{
			Name:       membs[i].Name,
			ID:         uint64(membs[i].ID),
			PeerURLs:   membs[i].PeerURLs,
			ClientURLs: membs[i].ClientURLs,
			IsLearner:  membs[i].IsLearner,
		}
	}
	return protoMembs
}
