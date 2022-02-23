/*
Copyright 2022.

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
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"

	"context"

	finalizerUtil "github.com/stakater/operator-utils/util/finalizer"
	reconcilerUtil "github.com/stakater/operator-utils/util/reconciler"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	sandboxv1alpha1 "github.com/example/sandbox-operator/api/v1alpha1"
)

// SandBoxReconciler reconciles a SandBox object
type SandBoxReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=sandbox.example.com,resources=sandboxes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sandbox.example.com,resources=sandboxes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sandbox.example.com,resources=sandboxes/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SandBox object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *SandBoxReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	log.Info("SandBox Reconciler called")

	sandBox := &sandboxv1alpha1.SandBox{}

	err := r.Get(ctx, req.NamespacedName, sandBox)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request
			return reconcilerUtil.DoNotRequeue()
		}
		return reconcilerUtil.RequeueWithError(err)
	}

	// sandBox is marked for deletion
	if sandBox.GetDeletionTimestamp() != nil {
		log.Info("Deletion timestamp found for sandBox")
		if finalizerUtil.HasFinalizer(sandBox, finalizer) {
			return r.finalizeSandBox(ctx, req, sandBox)
		}

		return reconcilerUtil.DoNotRequeue()
	}

	// adding finalizer in case it doesn't exist
	if !finalizerUtil.HasFinalizer(sandBox, finalizer) {
		log.Info("Adding finalizer to sandBox")

		finalizerUtil.AddFinalizer(sandBox, finalizer)

		err := r.Client.Update(ctx, sandBox)
		if err != nil {
			return reconcilerUtil.ManageError(r.Client, sandBox, err, true)
		}
	}

	if sandBox.Status.Name == "" {
		sandBox.Status.Name = sandBox.Spec.Name

		err = r.Status().Update(ctx, sandBox)
		if err != nil {
			log.Info("Sandbox status cannot be updated")
			return reconcilerUtil.ManageError(r.Client, sandBox, err, false)
		}
		r.createNamespace(ctx, req, sandBox)
	} else if sandBox.Spec.Name != sandBox.Status.Name {
		r.updateSandbox(ctx, sandBox, req)
	}

	return ctrl.Result{}, nil
}

func (r *SandBoxReconciler) finalizeSandBox(ctx context.Context, req ctrl.Request, sandBox *sandboxv1alpha1.SandBox) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Inside sandBox finalizer")

	if sandBox == nil {
		return reconcilerUtil.DoNotRequeue()
	}

	err := r.Client.Get(ctx, req.NamespacedName, sandBox)
	if err != nil {
		return reconcilerUtil.ManageError(r.Client, sandBox, err, false)
	}

	r.deleteNamespace(req, ctx, sandBox)

	finalizerUtil.DeleteFinalizer(sandBox, finalizer)
	log.Info("Finalizer removed for sandBox")

	err = r.Client.Update(context.Background(), sandBox)
	if err != nil {
		return reconcilerUtil.ManageError(r.Client, sandBox, err, false)
	}

	return reconcilerUtil.DoNotRequeue()
}

func (r *SandBoxReconciler) updateSandbox(ctx context.Context, sandBox *sandboxv1alpha1.SandBox, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Inside updateSandbox")

	if sandBox.Spec.Name != sandBox.Status.Name {
		log.Info("Name cannot be updated for sandbox")
		sandBox.Spec.Name = sandBox.Status.Name
		r.Client.Update(ctx, sandBox)
	}

	sandBox = &sandboxv1alpha1.SandBox{}
	err := r.Get(ctx, req.NamespacedName, sandBox)

	if err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Updating user details")
	return reconcilerUtil.ManageSuccess(r.Client, sandBox)
}

func (r *SandBoxReconciler) createNamespace(ctx context.Context, req ctrl.Request, sandBox *sandboxv1alpha1.SandBox) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Inside createNamespace")

	err := r.Get(ctx, req.NamespacedName, sandBox)
	if err != nil {
		log.Info("Cannot get sandBox")
	}

	nsSpec := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s%s", "ns-", strings.ToLower(sandBox.Spec.Name)),
		},
	}

	log.WithValues("Namespace", nsSpec)

	err = client.Client.Create(r.Client, ctx, nsSpec)

	if err != nil {
		if errors.IsAlreadyExists(err) {
			log.Info("Namespace already exists")
		} else {
			log.Info("Cannot create new namespace")
		}
		return reconcilerUtil.ManageError(r.Client, sandBox, err, true)
	}
	return ctrl.Result{}, nil
}

func (r *SandBoxReconciler) deleteNamespace(req ctrl.Request, ctx context.Context, sandBox *sandboxv1alpha1.SandBox) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Inside deleterNamespace")

	ns := &corev1.Namespace{}

	log.WithValues("Logging for namespace", ns)

	err := r.Client.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s%s", "ns-", strings.ToLower(sandBox.Spec.Name))}, ns)
	if err != nil {
		log.Info("Namespace not available")
		return ctrl.Result{}, err
	}

	err = r.Client.Delete(ctx, ns)
	if err != nil {
		log.Info("unable to delete the specified namespace in deleteNamespace function")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SandBoxReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sandboxv1alpha1.SandBox{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
