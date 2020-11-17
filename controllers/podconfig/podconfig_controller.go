/*


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

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	podconfigv1alpha1 "github.com/acmenezes/podconfig-operator/apis/podconfig/v1alpha1"
)

// PodConfigReconciler reconciles a PodConfig object
type PodConfigReconciler struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	podConfigList *podconfigv1alpha1.PodConfigList
	// podList       *corev1.PodList // TODO: needs a struct with a podlist for each podconfig
}

// +kubebuilder:rbac:groups=podconfig.opdev.io,resources=podconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=podconfig.opdev.io,resources=podconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=daemonsets;deployments;deployments/finalizers;replicasets,verbs=get;list;watch;create;update;patch;delete,namespace=cnf-test

// +kubebuilder:rbac:groups="*",resources="*",verbs="*"

// Reconcile function for the PodConfig instance
func (r *PodConfigReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	reqLogger := r.Log.WithName("podconfig-operator").WithValues("podconfig", req.NamespacedName)

	// List all pod configuration objects present on the namespace
	r.podConfigList = &podconfigv1alpha1.PodConfigList{}
	err := r.Client.List(context.TODO(), r.podConfigList)
	if err != nil {
		return reconcile.Result{}, err
	}

	if len(r.podConfigList.Items) <= 0 {
		return reconcile.Result{}, nil
	}
	// TODO: Update the status field with conditions while creating the new instance

	for _, podConfig := range r.podConfigList.Items {

		finalizer := "podconfig.finalizers.opdev.io"

		// examine DeletionTimestamp to determine if podConfig is under deletion
		if podConfig.ObjectMeta.DeletionTimestamp.IsZero() {

			// podConfig is not being deleted, so if it does not have our finalizer,
			// then lets add the finalizer and update the object. This is equivalent
			// registering our finalizer.

			if !containsString(podConfig.GetFinalizers(), finalizer) {
				podConfig.SetFinalizers(append(podConfig.GetFinalizers(), finalizer))
				if err := r.Update(context.Background(), &podConfig); err != nil {
					return reconcile.Result{}, err
				}
			}
		} else {
			// podConfig is being deleted
			if containsString(podConfig.GetFinalizers(), finalizer) {

				// finalizer is present, delete configurations

				// Get the pods with matching labels to podConfig
				podList, err := r.listPodsWithMatchingLabels(podConfig)
				if err != nil {
					return reconcile.Result{}, err
				}
				// Delete configuration defined in the podconfig CR from pods with the appropriate label.
				for _, pod := range podList.Items {

					if err := deleteConfig(pod, &podConfig); err != nil {
						// if fail to delete the external dependency here, return with error
						// so that it can be retried
						return reconcile.Result{}, err
					}
				}

				// remove our finalizer from the list and update it.
				podConfig.SetFinalizers(removeString(podConfig.GetFinalizers(), finalizer))
				if err := r.Update(context.Background(), &podConfig); err != nil {
					return reconcile.Result{}, err
				}
			}

			// Stop reconciliation as the item is being deleted
			return ctrl.Result{}, nil
		}

		if podConfig.Spec.SampleDeployment.Create {

			// Creates test deployments to PoC pod-to-pod communication over On demmand created Linux Veth Pairs

			err := r.createSampleDeployment(&podConfig, podConfig.Spec.SampleDeployment.Name, podConfig.ObjectMeta.Namespace, map[string]string{"podconfig": podConfig.ObjectMeta.Name})
			if err != nil {
				reqLogger.Error(err, "Failed to reconcile resource", "Name", "cnf-example", "Namespace", "cnf-test")
				return reconcile.Result{}, err
			}
		}

		podList, err := r.listPodsWithMatchingLabels(podConfig)
		if err != nil {
			return reconcile.Result{}, err
		}
		// Apply configuration defined in the podconfig CR to pods with the appropriate label.
		for _, pod := range podList.Items {

			// Pods need to be running in order to receive new configuration
			// Wait for pod phase running
			if pod.Status.Phase != "Running" {
				fmt.Printf("pod %v phase is %v, requeuing... ", pod.ObjectMeta.Name, pod.Status.Phase)
				return reconcile.Result{}, nil
			}

			configList, err := applyConfig(pod, &podConfig)
			if err != nil {
				fmt.Printf("%v", err)
				return reconcile.Result{}, nil
			}
			fmt.Printf("%v", configList)

			// Update config status for the actual pod in the list
			configStatus := podconfigv1alpha1.PodConfiguration{PodName: pod.ObjectMeta.Name, ConfigList: configList}
			fmt.Printf("%v", podConfig.Status.PodConfigurations)

			// Refresh cached object to avoid conflicts
			if err := r.Client.Get(context.TODO(), req.NamespacedName, &podConfig); err != nil {
				fmt.Printf("%v", err)
				return reconcile.Result{}, err
			}

			// If the pod config didn't reconcile completely update status
			if podConfig.Status.Phase != podconfigv1alpha1.PodConfigConfigured {

				isPodNamePresent := false

				for _, p := range podConfig.Status.PodConfigurations {
					if p.PodName == configStatus.PodName {
						isPodNamePresent = true
					}
				}
				if isPodNamePresent == false {

					podConfig.Status.PodConfigurations = append(podConfig.Status.PodConfigurations, configStatus)

					fmt.Printf("%v", podConfig.Status.PodConfigurations)

					if err := r.Client.Status().Update(context.TODO(), &podConfig); err != nil {
						fmt.Printf("%v", err)
						return reconcile.Result{}, err
					}
				}
			}
		}

		// All pods for that pod configuration (a.k.a. podConfig) have been configured
		// update general phase to configured
		if err := r.Client.Get(context.TODO(), req.NamespacedName, &podConfig); err != nil {
			fmt.Printf("%v", err)
			return reconcile.Result{}, err
		}
		podConfig.Status.Phase = podconfigv1alpha1.PodConfigConfigured
		if err := r.Client.Status().Update(context.TODO(), &podConfig); err != nil {
			fmt.Printf("%v", err)
			return reconcile.Result{}, err
		}

	}
	return reconcile.Result{}, nil
}

func (r *PodConfigReconciler) listPodsWithMatchingLabels(podConfig podconfigv1alpha1.PodConfig) (*corev1.PodList, error) {
	// Get the list of pods that have a podconfig label
	podList := &corev1.PodList{}
	err := r.Client.List(context.TODO(), podList, client.MatchingLabels{"podconfig": podConfig.ObjectMeta.Name})
	if err != nil {
		fmt.Println(err)
	}
	// Pods need to be at least created to proceed
	// Checking if the list is empty
	if len(podList.Items) <= 0 {
		return &corev1.PodList{}, fmt.Errorf("empty pod list")
	}
	return podList, nil
}

// SetupWithManager for the podconfig controller
func (r *PodConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&podconfigv1alpha1.PodConfig{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

// Helper functions to check and remove string from a slice of strings.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}
