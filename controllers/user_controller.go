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

	"k8s.io/apimachinery/pkg/api/errors"

	"context"

	finalizerUtil "github.com/stakater/operator-utils/util/finalizer"
	reconcilerUtil "github.com/stakater/operator-utils/util/reconciler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	userv1alpha1 "github.com/example/sandbox-operator/api/v1alpha1"
)

var (
	finalizer string = "finalizeUser"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=sandbox.example.com,resources=users,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sandbox.example.com,resources=users/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sandbox.example.com,resources=users/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the User object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("User Reconciler called")

	// Fetch the User instance
	user := &userv1alpha1.User{}
	err := r.Get(ctx, req.NamespacedName, user)

	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request
			return reconcilerUtil.DoNotRequeue()
		}
		// Error reading the object - requeue the request
		log.Error(err, "Failed to get User")
		return reconcilerUtil.RequeueWithError(err)
	}

	// user is marked for deletion
	if user.GetDeletionTimestamp() != nil {
		log.Info("Deletion timestamp found for user " + req.Name)
		if finalizerUtil.HasFinalizer(user, finalizer) {
			return r.finalizeUser(ctx, user, req)
		}
		// Finalizer doesn't exist so clean up is already done
		return reconcilerUtil.DoNotRequeue()
	}

	// adding finalizer if it doesn't exist already
	if !finalizerUtil.HasFinalizer(user, finalizer) {
		log.Info("Adding finalizer for user")

		finalizerUtil.AddFinalizer(user, finalizer)
		err = r.Client.Update(ctx, user)

		if err != nil {
			return reconcilerUtil.ManageError(r.Client, user, err, true)
		}
	}

	// creating a new user
	if user.Status.Name == "" {
		user.Status.Name = user.Spec.Name
		user.Status.SandBoxCount = user.Spec.SandBoxCount

		err = r.Status().Update(ctx, user)
		if err != nil {
			log.Info("Unable to update user status")
			return reconcilerUtil.ManageError(r.Client, user, err, false)
		}
		r.createSandBoxes(ctx, user, 1)
	} else if user.Spec.Name != user.Status.Name || user.Spec.SandBoxCount != user.Status.SandBoxCount {
		r.updateUser(ctx, user, req)
	}
	return ctrl.Result{}, nil
}

func (r *UserReconciler) createSandBoxes(ctx context.Context, user *userv1alpha1.User, a int8) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Inside createSandboxes")

	for i := a; i <= user.Spec.SandBoxCount; i++ {
		sandBox := &userv1alpha1.SandBox{

			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s%s%s%d", "sb-", strings.ToLower(user.Spec.Name), "-", i),
				Namespace: user.Namespace,
			},

			Spec: userv1alpha1.SandBoxSpec{
				Name: fmt.Sprintf("%s%s%s%d", "SB-", user.Spec.Name, "-", i),
				Type: "T1",
			},
		}

		err := r.Client.Create(ctx, sandBox)
		if err != nil {
			if errors.IsAlreadyExists(err) {
				log.Info("Sandbox already exists")
			} else {
				log.Info("Error occured while creating sandbox")
			}
			return reconcilerUtil.ManageError(r.Client, sandBox, err, true)
		}
	}

	return ctrl.Result{}, nil
}

func (r *UserReconciler) updateUser(ctx context.Context, user *userv1alpha1.User, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Inside updateUser")

	if user.Status.Name != user.Spec.Name {
		log.Info("User name cannot be updated")
	}

	if user.Spec.SandBoxCount != user.Status.SandBoxCount {
		if user.Spec.SandBoxCount < user.Status.SandBoxCount {
			log.Info("Spec count is less than status count, deleting old sandboxes")
		} else {
			log.Info("Spec count is greater than status count, creating new sandboxes")

			r.createSandBoxes(ctx, user, user.Spec.SandBoxCount+1)
			user.Status.SandBoxCount = user.Spec.SandBoxCount
			r.Client.Update(ctx, user)
		}
	}

	sandBox := &userv1alpha1.SandBox{}
	err := r.Get(ctx, req.NamespacedName, sandBox)

	if err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Updating user details")
	return reconcilerUtil.ManageSuccess(r.Client, user)
}

func (r *UserReconciler) deleteUser(ctx context.Context, user *userv1alpha1.User, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Inside deleteUser")

	sandBox := &userv1alpha1.SandBox{}
	userName := user.Spec.Name

	var i int8
	for i = 1; i <= user.Spec.SandBoxCount; i++ {

		err := r.Client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: fmt.Sprintf("%s%s%s%d", "sb-", strings.ToLower(user.Spec.Name), "-", i)}, sandBox)
		if err != nil {
			log.Info("Sandbox resources not available in deleteUser")
			return reconcilerUtil.ManageError(r.Client, sandBox, err, false)

		}
		log.Info("scope inside for loop to delete sandboxes")
		if strings.Contains(sandBox.Spec.Name, userName) {
			r.Client.Delete(ctx, sandBox)
		}
	}
	return ctrl.Result{}, nil
}

func (r *UserReconciler) finalizeUser(ctx context.Context, user *userv1alpha1.User, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Inside finalizeUser")

	if user == nil {
		return reconcilerUtil.DoNotRequeue()
	}

	log = log.WithValues("User Name: ", user)

	err := r.Client.Get(ctx, req.NamespacedName, user)

	if err != nil {
		return reconcilerUtil.ManageError(r.Client, user, err, false)
	}

	r.deleteUser(ctx, user, req)

	finalizerUtil.DeleteFinalizer(user, "finalizUser")
	log.Info("Finalizer removed")

	err = r.Client.Update(context.Background(), user)
	if err != nil {
		return reconcilerUtil.ManageError(r.Client, user, err, false)
	}
	return reconcilerUtil.DoNotRequeue()
}

// SetupWithManager sets up the controller with the Manager
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&userv1alpha1.User{}).
		Complete(r)
}
