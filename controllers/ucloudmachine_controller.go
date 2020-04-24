/*
Copyright 2019 The Kubernetes Authors.

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

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/ucloud/ucloud-sdk-go/services/uhost"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "sigs.k8s.io/cluster-api-provider-ucloud/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-ucloud/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-ucloud/cloud/services"
)

// UCloudMachineReconciler reconciles a UCloudMachine object
type UCloudMachineReconciler struct {
	client.Client
	Log logr.Logger
}

func (r *UCloudMachineReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.UCloudMachine{}).
		Watches(
			&source.Kind{Type: &clusterv1.Machine{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.MachineToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("UCloudMachine")),
			},
		).
		Watches(
			&source.Kind{Type: &infrav1.UCloudCluster{}},
			&handler.EnqueueRequestsFromMapFunc{ToRequests: handler.ToRequestsFunc(r.UCloudClusterToUCloudMachines)},
		).
		WithEventFilter(pausePredicates).
		Build(r)

	if err != nil {
		return err
	}

	return c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(r.requeueUCloudMachinesForUnpausedCluster),
		},
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldCluster := e.ObjectOld.(*clusterv1.Cluster)
				newCluster := e.ObjectNew.(*clusterv1.Cluster)
				return oldCluster.Spec.Paused && !newCluster.Spec.Paused
			},
			CreateFunc: func(e event.CreateEvent) bool {
				cluster := e.Object.(*clusterv1.Cluster)
				return !cluster.Spec.Paused
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return false
			},
		},
	)
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ucloudmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ucloudmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch

func (r *UCloudMachineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx := context.TODO()
	logger := r.Log.WithValues("namespace", req.Namespace, "ucloudMachine", req.Name)

	// Fetch the UCloudMachine instance.
	ucloudMachine := &infrav1.UCloudMachine{}
	err := r.Get(ctx, req.NamespacedName, ucloudMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, ucloudMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		logger.Info("Machine Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	logger = logger.WithValues("machine", machine.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		logger.Info("Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, nil
	}

	if isPaused(cluster, ucloudMachine) {
		logger.Info("UCloudMachine or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	logger = logger.WithValues("cluster", cluster.Name)

	ucloudCluster := &infrav1.UCloudCluster{}

	ucloudClusterName := client.ObjectKey{
		Namespace: ucloudMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, ucloudClusterName, ucloudCluster); err != nil {
		logger.Info("UCloudCluster is not available yet")
		return ctrl.Result{}, nil
	}

	logger = logger.WithValues("ucloudCluster", ucloudCluster.Name)

	// Create the cluster scope
	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Client:        r.Client,
		Logger:        logger,
		Cluster:       cluster,
		UCloudCluster: ucloudCluster,
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always close the scope when exiting this function so we can persist any Cluster changes.
	defer func() {
		if err := clusterScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Create the machine scope
	machineScope, err := scope.NewMachineScope(scope.MachineScopeParams{
		Logger:        logger,
		Client:        r.Client,
		Cluster:       cluster,
		Machine:       machine,
		UCloudCluster: ucloudCluster,
		UCloudMachine: ucloudMachine,
	})
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any UCloudMachine changes.
	defer func() {
		if err := machineScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted machines
	if !ucloudMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(machineScope, clusterScope)
	}

	// Handle non-deleted machines
	return r.reconcile(ctx, machineScope, clusterScope)
}

func (r *UCloudMachineReconciler) reconcile(ctx context.Context, machineScope *scope.MachineScope, clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	machineScope.Info("Reconciling UCloudMachine")
	// If the UCloudMachine is in an error state, return early.
	if machineScope.UCloudMachine.Status.FailureReason != nil || machineScope.UCloudMachine.Status.FailureMessage != nil {
		machineScope.Info("Error state detected, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// If the UCloudMachine doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(machineScope.UCloudMachine, infrav1.MachineFinalizer)
	if err := machineScope.PatchObject(); err != nil {
		return ctrl.Result{}, err
	}

	// aad FinalLizer for ucloudcluster
	controllerutil.AddFinalizer(clusterScope.UCloudCluster, machineScope.UCloudMachine.GetName()+".ucloudmachine")
	// Register the finalizer immediately to avoid orphaning ucloud resources on delete
	if err := clusterScope.PatchObject(); err != nil {
		return ctrl.Result{}, err
	}

	if !machineScope.Cluster.Status.InfrastructureReady {
		machineScope.Info("Cluster infrastructure is not ready yet")
		return ctrl.Result{}, nil
	}

	// Make sure bootstrap data is available and populated.
	if machineScope.Machine.Spec.Bootstrap.DataSecretName == nil {
		machineScope.Info("Bootstrap data secret reference is not yet available")
		return ctrl.Result{}, nil
	}

	computeSvc := services.NewService(clusterScope)

	// try to find instance
	instance, err := computeSvc.InstanceIfExists(machineScope)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to query UCloudMachine instance")
	}

	if instance == nil {
		// Set a failure message if we couldn't find the instance.
		if machineScope.GetInstanceID() != nil {
			machineScope.SetFailureReason(capierrors.UpdateMachineError)
			machineScope.SetFailureMessage(errors.New("uhost instance cannot be found, you may have deleted it manually"))
			machineScope.SetNotReady()

			if err := r.Delete(ctx, machineScope.Machine); err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		}
		// Create a new UCloudMachine instance if we couldn't find a running instance.
		instance, err = computeSvc.CreateInstance(machineScope)
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to create UCloudMachine instance")
		}
	}

	// Make sure Spec.ProviderID is always set.
	machineScope.SetProviderID(fmt.Sprintf("ucloud://%s/%s/%s", clusterScope.ProjectId(), instance.Zone, instance.UHostId))

	machineScope.SetZone(instance.Zone)
	machineScope.UCloudMachine.Status.InstanceId = instance.UHostId

	// Proceed to reconcile the UCloudMachine state.
	machineScope.SetInstanceStatus(string(instance.State))

	machineScope.SetAddresses(r.getAddresses(instance))

	switch uhost.State(instance.State) {
	case uhost.StateRunning:
		machineScope.Info("Machine instance is running", "instance-id", *machineScope.GetInstanceID())
		machineScope.SetReady()
		if machineScope.IsControlPlane() {
			if err := computeSvc.AddRealServer(instance.UHostId); err != nil {
				return ctrl.Result{}, err
			}
		}
	case uhost.StateInitializing, uhost.StateStarting:
		machineScope.Info("Machine instance is pending", "instance-id", *machineScope.GetInstanceID())
	case uhost.State(""):
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	default:
		machineScope.SetFailureReason(capierrors.UpdateMachineError)
		machineScope.SetFailureMessage(errors.Errorf("uhost instance %s state %q is unexpected", instance.UHostId, instance.State))
		machineScope.SetNotReady()
		if machineScope.IsControlPlane() {
			if err := computeSvc.DelRealServer(instance.UHostId); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	if err = computeSvc.CreateCAPUHost(machineScope); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create uk8s capu host")
	}

	return ctrl.Result{}, nil
}

func (r *UCloudMachineReconciler) reconcileDelete(machineScope *scope.MachineScope, clusterScope *scope.ClusterScope) (_ ctrl.Result, reterr error) {
	machineScope.Info("Handling delete UCloudMachine")

	computeSvc := services.NewService(clusterScope)

	// try to find instance
	instance, err := computeSvc.InstanceIfExists(machineScope)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to query UCloudMachine instance")
	}

	if instance == nil {
		// The machine was never created or was deleted by some other entity
		machineScope.V(3).Info("Unable to locate instance by ID or tags")
	} else {

		if machineScope.IsControlPlane() {
			machineScope.Info("removing instance from ulb backend")
			if err := computeSvc.DelRealServer(instance.UHostId); err != nil {
				return ctrl.Result{}, errors.Wrapf(err, "failed to terminate instance %s", instance.UHostId)
			}
		}

		if err = computeSvc.DeleteCAPUHost(machineScope); err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to delete uk8s capu host")
		}

		machineScope.Info("Terminating instance")
		if err := computeSvc.TerminateInstanceAndWait(machineScope); err != nil {
			record.Warnf(machineScope.UCloudMachine, "FailedTerminate", "Failed to terminate instance %q: %v", instance.UHostId, err)
			return ctrl.Result{}, errors.Errorf("failed to terminate instance: %+v", err)
		}

		record.Eventf(machineScope.UCloudMachine, "SuccessfulTerminate", "Terminated instance %q", instance.UHostId)
	}

	// Instance is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(machineScope.UCloudMachine, infrav1.MachineFinalizer)

	return ctrl.Result{}, nil
}

func (r *UCloudMachineReconciler) getAddresses(instance *uhost.UHostInstanceSet) []clusterv1.MachineAddress {
	addresses := make([]clusterv1.MachineAddress, 0, len(instance.IPSet))
	for _, nic := range instance.IPSet {
		var addressType clusterv1.MachineAddressType
		switch nic.Type {
		case "Internation", "Bgp":
			addressType = clusterv1.MachineExternalIP
		case "Private":
			addressType = clusterv1.MachineInternalIP
		}
		internalAddress := clusterv1.MachineAddress{
			Type:    addressType,
			Address: nic.IP,
		}
		addresses = append(addresses, internalAddress)
	}

	return addresses
}

// UCloudClusterToUCloudMachine is a handler.ToRequestsFunc to be used to enqeue requests for reconciliation
// of UCloudMachines.
func (r *UCloudMachineReconciler) UCloudClusterToUCloudMachines(o handler.MapObject) []ctrl.Request {
	c, ok := o.Object.(*infrav1.UCloudCluster)
	if !ok {
		r.Log.Error(errors.Errorf("expected a UCloudCluster but got a %T", o.Object), "failed to get UCloudMachine for UCloudCluster")
		return nil
	}
	log := r.Log.WithValues("UCloudCluster", c.Name, "Namespace", c.Namespace)

	cluster, err := util.GetOwnerCluster(context.TODO(), r.Client, c.ObjectMeta)
	switch {
	case apierrors.IsNotFound(err) || cluster == nil:
		return nil
	case err != nil:
		log.Error(err, "failed to get owning cluster")
		return nil
	}

	return r.requestsForCluster(cluster.Namespace, cluster.Name)
}

func (r *UCloudMachineReconciler) requeueUCloudMachinesForUnpausedCluster(o handler.MapObject) []ctrl.Request {
	c, ok := o.Object.(*clusterv1.Cluster)
	if !ok {
		r.Log.Error(errors.Errorf("expected a Cluster but got a %T", o.Object), "failed to get UCloudMachines for unpaused Cluster")
		return nil
	}

	// Don't handle deleted clusters
	if !c.ObjectMeta.DeletionTimestamp.IsZero() {
		return nil
	}

	return r.requestsForCluster(c.Namespace, c.Name)
}

func (r *UCloudMachineReconciler) requestsForCluster(namespace, name string) []ctrl.Request {
	log := r.Log.WithValues("Cluster", name, "Namespace", namespace)
	labels := map[string]string{clusterv1.ClusterLabelName: name}
	machineList := &clusterv1.MachineList{}
	if err := r.Client.List(context.TODO(), machineList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
		log.Error(err, "failed to get owned Machines")
		return nil
	}

	result := make([]ctrl.Request, 0, len(machineList.Items))
	for _, m := range machineList.Items {
		if m.Spec.InfrastructureRef.Name != "" {
			result = append(result, ctrl.Request{NamespacedName: client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.InfrastructureRef.Name}})
		}
	}
	return result
}
