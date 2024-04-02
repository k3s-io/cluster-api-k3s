/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package k3s

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/k3s-io/cluster-api-k3s/pkg/etcd"
	etcdutil "github.com/k3s-io/cluster-api-k3s/pkg/etcd/util"
)

const (
	EtcdRemoveAnnotation      = "etcd.k3s.cattle.io/remove"
	EtcdRemovedNodeAnnotation = "etcd.k3s.cattle.io/removed-node-name"
)

type etcdClientFor interface {
	forFirstAvailableNode(ctx context.Context, nodeNames []string) (*etcd.Client, error)
	forLeader(ctx context.Context, nodeNames []string) (*etcd.Client, error)
}

// ReconcileEtcdMembers iterates over all etcd members and finds members that do not have corresponding nodes.
// If there are any such members, it deletes them from etcd so that k3s controlplane does not run etcd health checks on them.
func (w *Workload) ReconcileEtcdMembers(ctx context.Context, nodeNames []string) ([]string, error) {
	allRemovedMembers := []string{}
	allErrs := []error{}
	for _, nodeName := range nodeNames {
		removedMembers, errs := w.reconcileEtcdMember(ctx, nodeNames, nodeName)
		allRemovedMembers = append(allRemovedMembers, removedMembers...)
		allErrs = append(allErrs, errs...)
	}

	return allRemovedMembers, kerrors.NewAggregate(allErrs)
}

func (w *Workload) reconcileEtcdMember(ctx context.Context, nodeNames []string, nodeName string) ([]string, []error) {
	log := ctrl.LoggerFrom(ctx)

	// Create the etcd Client for the etcd Pod scheduled on the Node
	etcdClient, err := w.etcdClientGenerator.forFirstAvailableNode(ctx, []string{nodeName})
	if err != nil {
		return nil, nil
	}
	defer etcdClient.Close()

	members, err := etcdClient.Members(ctx)
	if err != nil {
		return nil, nil
	}

	// Check if any member's node is missing from workload cluster
	// If any, delete it with best effort
	removedMembers := []string{}
	errs := []error{}
loopmembers:
	for _, member := range members {
		curNodeName := etcdutil.NodeNameFromMember(member)

		for _, nodeName := range nodeNames {
			if curNodeName == nodeName {
				// We found the matching node, continue with the outer loop.
				continue loopmembers
			}
		}

		// If we're here, the node cannot be found.
		removedMembers = append(removedMembers, curNodeName)
		log.Info("removing etcd from nonexisting node", "node", curNodeName)
		if err := w.removeMemberForNonExistingNode(ctx, curNodeName); err != nil {
			errs = append(errs, err)
		}
	}
	return removedMembers, errs
}

// RemoveEtcdMemberForMachine removes the etcd member from the target cluster's etcd cluster.
// Removing the last remaining member of the cluster is not supported.
func (w *Workload) RemoveEtcdMemberForMachine(ctx context.Context, machine *clusterv1.Machine) (bool, error) {
	if machine == nil || machine.Status.NodeRef == nil {
		// Nothing to do, no node for Machine
		return true, nil
	}
	return w.removeMemberForNode(ctx, machine.Status.NodeRef.Name)
}

func (w *Workload) removeMemberForNonExistingNode(ctx context.Context, name string) error {
	controlPlaneNodes, err := w.getControlPlaneNodes(ctx)
	if err != nil {
		return err
	}
	if len(controlPlaneNodes.Items) < 2 {
		return ErrControlPlaneMinNodes
	}

	var remainingNodes []string
	for _, n := range controlPlaneNodes.Items {
		if n.Name != name {
			remainingNodes = append(remainingNodes, n.Name)
		}
	}

	etcdClient, err := w.etcdClientGenerator.forFirstAvailableNode(ctx, remainingNodes)
	if err != nil {
		return errors.Wrap(err, "failed to create etcd client")
	}
	defer etcdClient.Close()

	// List etcd members. This checks that the member is healthy, because the request goes through consensus.
	members, err := etcdClient.Members(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list etcd members using etcd client")
	}
	member := etcdutil.MemberForName(members, name)

	// The member has already been removed, return immediately
	if member == nil {
		return nil
	}

	if err := etcdClient.RemoveMember(ctx, member.ID); err != nil {
		return errors.Wrap(err, "failed to remove member from etcd")
	}

	return nil
}

func (w *Workload) removeMemberForNode(ctx context.Context, name string) (bool, error) {
	controlPlaneNodes, err := w.getControlPlaneNodes(ctx)
	if err != nil {
		return false, err
	}
	if len(controlPlaneNodes.Items) < 2 {
		return false, ErrControlPlaneMinNodes
	}

	var removingNode corev1.Node
	for _, n := range controlPlaneNodes.Items {
		if n.Name == name {
			removingNode = n
		}
	}

	if removingNode.Name != name {
		return false, errors.New(fmt.Sprintf("node %s not found", name))
	}

	annotations := removingNode.GetAnnotations()
	if _, ok := annotations[EtcdRemovedNodeAnnotation]; ok {
		return true, nil
	}

	patchHelper, err := patch.NewHelper(&removingNode, w.Client)
	if err != nil {
		return false, errors.Wrapf(err, "failed to create patch helper for node")
	}

	annotations[EtcdRemoveAnnotation] = "true"
	removingNode.SetAnnotations(annotations)
	if err := patchHelper.Patch(ctx, &removingNode); err != nil {
		return false, errors.Wrapf(err, "failed patch node")
	}

	return false, nil
}

// ForwardEtcdLeadership forwards etcd leadership to the first follower.
func (w *Workload) ForwardEtcdLeadership(ctx context.Context, machine *clusterv1.Machine, leaderCandidate *clusterv1.Machine) error {
	if machine == nil || machine.Status.NodeRef == nil {
		return nil
	}
	if leaderCandidate == nil {
		return errors.New("leader candidate cannot be nil")
	}
	if leaderCandidate.Status.NodeRef == nil {
		return errors.New("leader has no node reference")
	}

	nodes, err := w.getControlPlaneNodes(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list control plane nodes")
	}
	nodeNames := make([]string, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		nodeNames = append(nodeNames, node.Name)
	}
	etcdClient, err := w.etcdClientGenerator.forLeader(ctx, nodeNames)
	if err != nil {
		return errors.Wrap(err, "failed to create etcd client")
	}
	defer etcdClient.Close()

	members, err := etcdClient.Members(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list etcd members using etcd client")
	}

	currentMember := etcdutil.MemberForName(members, machine.Status.NodeRef.Name)
	if currentMember == nil || currentMember.ID != etcdClient.LeaderID {
		// nothing to do, this is not the etcd leader
		return nil
	}

	// Move the leader to the provided candidate.
	nextLeader := etcdutil.MemberForName(members, leaderCandidate.Status.NodeRef.Name)
	if nextLeader == nil {
		return errors.Errorf("failed to get etcd member from node %q", leaderCandidate.Status.NodeRef.Name)
	}
	if err := etcdClient.MoveLeader(ctx, nextLeader.ID); err != nil {
		return errors.Wrapf(err, "failed to move leader")
	}
	return nil
}
