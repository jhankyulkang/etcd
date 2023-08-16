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

package command

import (
	"errors"
	"fmt"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/cobrautl"
)

var (
	memberPeerURLs string
	isLearner      bool
	explictLeave   bool
	leave          bool
	add            string
	remove         string
	quroum         int
)

// NewMemberCommand returns the cobra command for "member".
func NewMemberCommand() *cobra.Command {
	mc := &cobra.Command{
		Use:   "member <subcommand>",
		Short: "Membership related commands",
	}

	mc.AddCommand(NewMemberAddCommand())
	mc.AddCommand(NewMemberRemoveCommand())
	mc.AddCommand(NewMemberUpdateCommand())
	mc.AddCommand(NewMemberListCommand())
	mc.AddCommand(NewMemberPromoteCommand())
	mc.AddCommand(NewMemberSplitCommand())
	mc.AddCommand(NewMemberMergeCommand())
	mc.AddCommand(NewMemberJointCommand())

	return mc
}

// NewMemberAddCommand returns the cobra command for "member add".
func NewMemberAddCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "add <memberName> [options]",
		Short: "Adds a member into the cluster",

		Run: memberAddCommandFunc,
	}

	cc.Flags().StringVar(&memberPeerURLs, "peer-urls", "", "comma separated peer URLs for the new member.")
	//cc.Flags().StringSliceVar(&addMemberPeerURLs, "peer-urls", nil, "comma separated peer URLs for the new member.")
	//todo: not mapped per member!
	cc.Flags().BoolVar(&isLearner, "learner", false, "indicates if the new member is raft learner")

	cc.Flags().IntVar(&quroum, "quorum", 0, "manually set dynamic quorum")
	return cc
}

// NewMemberRemoveCommand returns the cobra command for "member remove".
func NewMemberRemoveCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "remove <memberID>",
		Short: "Removes a member from the cluster",

		Run: memberRemoveCommandFunc,
	}

	return cc
}

// NewMemberUpdateCommand returns the cobra command for "member update".
func NewMemberUpdateCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "update <memberID> [options]",
		Short: "Updates a member in the cluster",

		Run: memberUpdateCommandFunc,
	}

	cc.Flags().StringVar(&memberPeerURLs, "peer-urls", "", "comma separated peer URLs for the updated member.")

	return cc
}

// NewMemberListCommand returns the cobra command for "member list".
func NewMemberListCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "list",
		Short: "Lists all members in the cluster",
		Long: `When --write-out is set to simple, this command prints out comma-separated member lists for each endpoint.
The items in the lists are ID, Progress, Name, Peer Addrs, Client Addrs, Is Learner.
`,

		Run: memberListCommandFunc,
	}

	return cc
}

// NewMemberPromoteCommand returns the cobra command for "member promote".
func NewMemberPromoteCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "promote <memberID>",
		Short: "Promotes a non-voting member in the cluster",
		Long: `Promotes a non-voting learner member to a voting one in the cluster.
`,

		Run: memberPromoteCommandFunc,
	}

	return cc
}

// NewMemberSplitCommand returns the cobra command for "member split".
func NewMemberSplitCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "split <memberIDs>",
		Short: "Split members (comma seperated IDs) from the cluster",
		Long: `Split members from the cluster.
`,

		Run: memberSplitCommandFunc,
	}

	cc.Flags().BoolVar(&explictLeave, "explict-leave", false, "indicates if members should leave split joint consensus explicitly")
	cc.Flags().BoolVar(&leave, "leave", false, "indicates if members should leave split joint consensus")

	return cc
}

// NewMemberMergeCommand returns the cobra command for "member merge".
func NewMemberMergeCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "merge <memberEndpoints>",
		Short: "Merge clusters (identified by member endpoints) into this one",
		Long: `Merge clusters into this one.
`,

		Run: memberMergeCommandFunc,
	}

	return cc
}

// NewMemberJointCommand returns the cobra command for "member joint".
func NewMemberJointCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "joint --add <memberPeerUrls> --remove <memberIDs>",
		Short: "Add or remove members with joint consensus",
		Long: `Add or remove members with joint consensus 
`,

		Run: memberJointCommandFunc,
	}

	cc.Flags().StringVar(&add, "add", "", "comma seperated urls for nodes to add, one peer url for one node")
	cc.Flags().StringVar(&remove, "remove", "", "comma seperated IDs for nodes to remove")

	return cc
}

// memberAddCommandFunc executes the "member add" command.
func memberAddCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, errors.New("member name not provided"))
	}
	if len(memberPeerURLs) == 0 {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, errors.New("member peer urls not provided"))
	}
	var (
		resp *clientv3.MemberAddResponse
		err  error
	)
	peerURLs := strings.Split(memberPeerURLs, "+++")
	//fmt.Print(peerURLs)
	//conf := []string{}
	for i, s := range args {
		if strings.HasPrefix(strings.ToLower(s), "http") {
			ev := fmt.Sprintf(`, did you mean --peer-urls=%s`, s)
			cobrautl.ExitWithError(cobrautl.ExitBadArgs, errors.New(ev))
		}

		memberPeerURLs := strings.Split(peerURLs[i], ",")

		//newMemberName := args[0]

		ctx, cancel := commandCtx(cmd)
		cli := mustClientFromCmd(cmd)

		if isLearner {
			resp, err = cli.MemberAddAsLearner(ctx, memberPeerURLs, uint64(quroum))
		} else {
			fmt.Print(quroum)
			resp, err = cli.MemberAdd(ctx, memberPeerURLs, uint64(quroum))
		}
		cancel()
		if resp != nil && err != nil {
			//cobrautl.ExitWithError(cobrautl.ExitError, err)
		}

		/*newID := resp.Member.ID
		display.MemberAdd(*resp)
		if _, ok := (display).(*simplePrinter); ok {

			for _, memb := range resp.Members {
				for _, u := range memb.PeerURLs {
					n := memb.Name
					if memb.ID == newID {
						n = s
					}
					conf = append(conf, fmt.Sprintf("%s=%s", n, u))
				}
			}
		}

		fmt.Print("\n")
		//fmt.Printf("ETCD_NAME=%q\n", newMemberName)
		fmt.Printf("ETCD_INITIAL_CLUSTER=%q\n", strings.Join(conf, ","))
		fmt.Printf("ETCD_INITIAL_ADVERTISE_PEER_URLS=%q\n", memberPeerURLs)
		fmt.Printf("ETCD_INITIAL_CLUSTER_STATE=\"existing\"\n")
		//}*/
	}
}

// memberRemoveCommandFunc executes the "member remove" command.
func memberRemoveCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("member ID is not provided"))
	}
	for i := range args {
		id, err := strconv.ParseUint(args[i], 16, 64)
		if err != nil {
			cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("bad member ID arg (%v), expecting ID in Hex", err))
		}

		ctx, cancel := commandCtx(cmd)
		resp, err := mustClientFromCmd(cmd).MemberRemove(ctx, id)
		cancel()
		/*if err != nil {
			cobrautl.ExitWithError(cobrautl.ExitError, err)
		}*/
		display.MemberRemove(id, *resp)
	}
}

// memberUpdateCommandFunc executes the "member update" command.
func memberUpdateCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("member ID is not provided"))
	}

	id, err := strconv.ParseUint(args[0], 16, 64)
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("bad member ID arg (%v), expecting ID in Hex", err))
	}

	if len(memberPeerURLs) == 0 {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("member peer urls not provided"))
	}

	urls := strings.Split(memberPeerURLs, ",")

	ctx, cancel := commandCtx(cmd)
	resp, err := mustClientFromCmd(cmd).MemberUpdate(ctx, id, urls)
	cancel()
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}

	display.MemberUpdate(id, *resp)
}

// memberListCommandFunc executes the "member list" command.
func memberListCommandFunc(cmd *cobra.Command, args []string) {
	ctx, cancel := commandCtx(cmd)
	resp, err := mustClientFromCmd(cmd).MemberList(ctx)
	cancel()
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}

	display.MemberList(*resp)
}

// memberPromoteCommandFunc executes the "member promote" command.
func memberPromoteCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("member ID is not provided"))
	}

	id, err := strconv.ParseUint(args[0], 16, 64)
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("bad member ID arg (%v), expecting ID in Hex", err))
	}

	ctx, cancel := commandCtx(cmd)
	resp, err := mustClientFromCmd(cmd).MemberPromote(ctx, id)
	cancel()
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}
	display.MemberPromote(id, *resp)
}

func memberSplitCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("member IDs are not provided"))
	}

	clrs := make([]etcdserverpb.MemberList, 0)
	for _, clrStr := range args {
		idStrs := strings.Split(clrStr, ",")
		mems := make([]etcdserverpb.Member, 0)
		for _, idStr := range idStrs {
			id, err := strconv.ParseUint(idStr, 16, 64)
			if err != nil {
				cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("bad member ID arg (%v), expecting ID in Hex", err))
			}
			mems = append(mems, etcdserverpb.Member{ID: id})
		}
		clrs = append(clrs, etcdserverpb.MemberList{Members: mems})
	}

	ctx, cancel := commandCtx(cmd)
	_, err := mustClientFromCmd(cmd).MemberSplit(ctx, clrs, explictLeave, leave)
	cancel()
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}
}

func memberMergeCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("member URLs are not provided"))
	}

	ctx, cancel := commandCtx(cmd)

	urlStrs := strings.Split(args[0], ",")
	clusters := map[uint64]etcdserverpb.MemberList{}
	for _, url := range urlStrs {
		client, err := clientv3.New(clientv3.Config{Endpoints: []string{url}})
		if err != nil {
			cobrautl.ExitWithError(cobrautl.ExitBadArgs,
				fmt.Errorf("cannot create client by url (%v): %v", url, err))
		}

		resp, err := client.MemberList(ctx)
		if err != nil {
			cobrautl.ExitWithError(cobrautl.ExitBadArgs,
				fmt.Errorf("cannot list member by url (%v): %v", url, err))
		}
		if len(resp.Members) == 0 {
			cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("no member exists by url (%v)", url))
		}

		mems := make([]etcdserverpb.Member, 0)
		for _, mem := range resp.Members {
			mems = append(mems, *mem)
		}
		clusters[resp.Header.ClusterId] = etcdserverpb.MemberList{Members: mems}
	}

	_, err := mustClientFromCmd(cmd).MemberMerge(ctx, clusters)
	cancel()
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}
}

func memberJointCommandFunc(cmd *cobra.Command, args []string) {
	if len(add) == 0 && len(remove) == 0 {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("no members provided"))
	}

	addUrls := make([]string, 0)
	if len(add) != 0 {
		addUrls = strings.Split(add, ",")
	}

	removeIds := make([]uint64, 0)
	if len(remove) != 0 {
		for _, idStr := range strings.Split(remove, ",") {
			id, err := strconv.ParseUint(idStr, 16, 64)
			if err != nil {
				cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("bad member ID arg (%v), expecting ID in Hex", err))
			}
			removeIds = append(removeIds, id)
		}
	}

	ctx, cancel := commandCtx(cmd)
	_, err := mustClientFromCmd(cmd).MemberJoint(ctx, addUrls, removeIds)
	cancel()
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}
}
