package k3s

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/controllers/remote"

	"github.com/zawachte-msft/cluster-api-k3s/pkg/machinefilters"

	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagementCluster defines all behaviors necessary for something to function as a management cluster.
type ManagementCluster interface {
	ctrlclient.Reader

	GetMachinesForCluster(ctx context.Context, cluster client.ObjectKey, filters ...machinefilters.Func) (FilterableMachineCollection, error)
	GetWorkloadCluster(ctx context.Context, clusterKey client.ObjectKey) (WorkloadCluster, error)
}

// Management holds operations on the management cluster.
type Management struct {
	Client ctrlclient.Reader
}

// RemoteClusterConnectionError represents a failure to connect to a remote cluster
type RemoteClusterConnectionError struct {
	Name string
	Err  error
}

func (e *RemoteClusterConnectionError) Error() string { return e.Name + ": " + e.Err.Error() }
func (e *RemoteClusterConnectionError) Unwrap() error { return e.Err }

// Get implements ctrlclient.Reader
func (m *Management) Get(ctx context.Context, key ctrlclient.ObjectKey, obj runtime.Object) error {
	return m.Client.Get(ctx, key, obj)
}

// List implements ctrlclient.Reader
func (m *Management) List(ctx context.Context, list runtime.Object, opts ...ctrlclient.ListOption) error {
	return m.Client.List(ctx, list, opts...)
}

// GetMachinesForCluster returns a list of machines that can be filtered or not.
// If no filter is supplied then all machines associated with the target cluster are returned.
func (m *Management) GetMachinesForCluster(ctx context.Context, cluster client.ObjectKey, filters ...machinefilters.Func) (FilterableMachineCollection, error) {
	selector := map[string]string{
		clusterv1.ClusterLabelName: cluster.Name,
	}
	ml := &clusterv1.MachineList{}
	if err := m.Client.List(ctx, ml, client.InNamespace(cluster.Namespace), client.MatchingLabels(selector)); err != nil {
		return nil, errors.Wrap(err, "failed to list machines")
	}

	machines := NewFilterableMachineCollectionFromMachineList(ml)
	return machines.Filter(filters...), nil
}

// GetWorkloadCluster builds a cluster object.
// The cluster comes with an etcd client generator to connect to any etcd pod living on a managed machine.
func (m *Management) GetWorkloadCluster(ctx context.Context, clusterKey client.ObjectKey) (WorkloadCluster, error) {
	restConfig, err := remote.RESTConfig(ctx, m.Client, clusterKey)
	if err != nil {
		return nil, err
	}
	restConfig.Timeout = 30 * time.Second

	c, err := client.New(restConfig, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, &RemoteClusterConnectionError{Name: clusterKey.String(), Err: err}
	}

	return &Workload{
		Client:          c,
		CoreDNSMigrator: &CoreDNSMigrator{},
	}, nil
}
