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
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
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

// UCloudClusterReconciler reconciles a UCloudCluster object
type UCloudClusterReconciler struct {
	client.Client
	Log logr.Logger
}

func (r *UCloudClusterReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.UCloudCluster{}).
		WithEventFilter(pausePredicates).
		Build(r)
	if err != nil {
		return errors.Wrap(err, "error creating controller")
	}

	return c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(r.requeueUCloudClusterForUnpausedCluster),
		},
		predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				cluster := e.Object.(*clusterv1.Cluster)
				return !cluster.Spec.Paused
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldCluster := e.ObjectOld.(*clusterv1.Cluster)
				newCluster := e.ObjectNew.(*clusterv1.Cluster)
				return oldCluster.Spec.Paused && !newCluster.Spec.Paused
			},
		},
	)
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ucloudclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ucloudclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

func (r *UCloudClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx := context.TODO()
	log := r.Log.WithValues("namespace", req.Namespace, "ucloudCluster", req.Name)

	// Fetch the UCloudCluster instance
	ucloudCluster := &infrav1.UCloudCluster{}
	err := r.Get(ctx, req.NamespacedName, ucloudCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, ucloudCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}

	if isPaused(cluster, ucloudCluster) {
		log.Info("UCloudCluster of linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	if cluster == nil {
		log.Info("Cluster Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	// Create the scope.
	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Client:        r.Client,
		Logger:        log,
		Cluster:       cluster,
		UCloudCluster: ucloudCluster,
	})
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist changes.
	defer func() {
		if err := clusterScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted clusters
	if !ucloudCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(clusterScope)
	}

	// Handle non-deleted clusters
	return r.reconcile(clusterScope)
}

func (r *UCloudClusterReconciler) reconcile(clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	clusterScope.Info("Reconciling UCloudCluster")

	ucloudCluster := clusterScope.UCloudCluster

	// If the UCloudCluster doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(ucloudCluster, infrav1.ClusterFinalizer)
	// Register the finalizer immediately to avoid orphaning ucloud resources on delete
	if err := clusterScope.PatchObject(); err != nil {
		return ctrl.Result{}, err
	}

	computeSvc := services.NewService(clusterScope)

	if err := computeSvc.ReconcileUGroup(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile group for UCloudCluster %s/%s", ucloudCluster.Namespace, ucloudCluster.Name)
	}

	if err := computeSvc.ReconcileVPC(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile network for UCloudCluster %s/%s", ucloudCluster.Namespace, ucloudCluster.Name)
	}

	if err := computeSvc.ReconcileSubnet(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile subnet for UCloudCluster %s/%s", ucloudCluster.Namespace, ucloudCluster.Name)
	}

	if err := computeSvc.ReconcileNat(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile nat gateway for UCloudCluster %s/%s", ucloudCluster.Namespace, ucloudCluster.Name)
	}

	if err := computeSvc.ReconcileULB(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile load balancers for UCloudCluster %s/%s", ucloudCluster.Namespace, ucloudCluster.Name)
	}

	if ucloudCluster.Status.Network.ULB.EIP.EIPAddr == "" {
		clusterScope.Info("Waiting on API server Global IP Address")
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	// Set APIEndpoints so the Cluster API Cluster Controller can pull them
	ucloudCluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: ucloudCluster.Status.Network.ULB.EIP.EIPAddr,
		Port: 6443,
	}

	// Set FailureDomains on the UCloudCluster Status
	zones, err := computeSvc.GetZones()
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to get available zones for UCloudCluster %s/%s", ucloudCluster.Namespace, ucloudCluster.Name)
	}
	ucloudCluster.Status.FailureDomains = make(clusterv1.FailureDomains, len(zones))
	for _, zone := range zones {
		ucloudCluster.Status.FailureDomains[zone] = clusterv1.FailureDomainSpec{
			ControlPlane: true,
		}
	}

	// No errors, so mark us ready so the Cluster API Cluster Controller can pull it
	ucloudCluster.Status.Ready = true

	if err = computeSvc.CreateBastionInstance(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create bastion instance for UCloudCluster %s/%s", ucloudCluster.Namespace, ucloudCluster.Name)
	}

	if err = computeSvc.CreateCAPUCluster(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create uk8s capu cluster for UCloudCluster %s/%s", ucloudCluster.Namespace, ucloudCluster.Name)
	}

	return ctrl.Result{}, nil
}

func (r *UCloudClusterReconciler) reconcileDelete(clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	clusterScope.Info("Reconciling UCloudCluster delete")
	ctx := context.TODO()

	finalizers := clusterScope.UCloudCluster.GetFinalizers()
	for _, finalizer := range finalizers {
		if strings.HasSuffix(finalizer, ".ucloudmachine") {
			ucloudMachine := &infrav1.UCloudMachine{}
			ucloudMachineName := client.ObjectKey{
				Namespace: clusterScope.Namespace(),
				Name:      strings.Split(finalizer, ".")[0],
			}
			err := r.Client.Get(ctx, ucloudMachineName, ucloudMachine)
			if apierrors.IsNotFound(err) {
				// Cluster is deleted so remove the finalizer.
				controllerutil.RemoveFinalizer(clusterScope.UCloudCluster, finalizer)
			} else {
				clusterScope.Info("Waiting on all machine deleted")
				return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
			}
		}
	}

	computeSvc := services.NewService(clusterScope)
	ucloudCluster := clusterScope.UCloudCluster

	if err := computeSvc.TerminateBastion(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "error deleting bastion for UCloudCluster %s/%s", ucloudCluster.Namespace, ucloudCluster.Name)
	}

	if err := computeSvc.DeleteULB(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "error deleting load balancer for UCloudCluster %s/%s", ucloudCluster.Namespace, ucloudCluster.Name)
	}

	if err := computeSvc.DeleteNat(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "error deleting nat gateway for UCloudCluster %s/%s", ucloudCluster.Namespace, ucloudCluster.Name)
	}

	if err := computeSvc.CleanResourceInGroup(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "error cleaning resource in business group for UCloudCluster %s/%s", ucloudCluster.Namespace, ucloudCluster.Name)
	}

	if err := computeSvc.DeleteSubnet(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "error deleting vpc subnet for UCloudCluster %s/%s", ucloudCluster.Namespace, ucloudCluster.Name)
	}

	if err := computeSvc.DeleteVPC(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "error deleting vpc for UCloudCluster %s/%s", ucloudCluster.Namespace, ucloudCluster.Name)
	}

	if err := computeSvc.DeleteGroup(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "error deleting group for UCloudCluster %s/%s", ucloudCluster.Namespace, ucloudCluster.Name)
	}

	if err := computeSvc.DeleteCAPUCluster(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to delete uk8s capu cluster for UCloudCluster %s/%s", ucloudCluster.Namespace, ucloudCluster.Name)
	}

	// Cluster is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(clusterScope.UCloudCluster, infrav1.ClusterFinalizer)

	return ctrl.Result{}, nil
}

func (r *UCloudClusterReconciler) requeueUCloudClusterForUnpausedCluster(o handler.MapObject) []ctrl.Request {
	c, ok := o.Object.(*clusterv1.Cluster)
	if !ok {
		r.Log.Error(errors.Errorf("expected a Cluster but got a %T", o.Object), "failed to get UCloudClusters for unpaused Cluster")
		return nil
	}

	// Don't handle deleted clusters
	if !c.ObjectMeta.DeletionTimestamp.IsZero() {
		return nil
	}

	// Make sure the ref is set
	if c.Spec.InfrastructureRef == nil {
		return nil
	}

	return []ctrl.Request{
		{
			NamespacedName: client.ObjectKey{Namespace: c.Namespace, Name: c.Spec.InfrastructureRef.Name},
		},
	}
}
