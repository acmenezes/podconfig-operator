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
	Log    logr.Logger
	Scheme *runtime.Scheme
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
	podConfigList := &podconfigv1alpha1.PodConfigList{}
	err := r.Client.List(context.TODO(), podConfigList)
	if err != nil {
		return reconcile.Result{}, err
	}

	if len(podConfigList.Items) <= 0 {
		return reconcile.Result{}, nil
	}
	// TODO: Update the status field with conditions while creating the new instance

	// Deployments to mockup CNFs on OpenShift Cluster
	// Creating deployment with 2 replicas to simulate CNF-to-CNF communication over Linux VLANs

	for _, podConfig := range podConfigList.Items {

		if podConfig.Spec.SampleDeployment.Create {

			err := r.createSampleDeployment(&podConfig, podConfig.Spec.SampleDeployment.Name, podConfig.ObjectMeta.Namespace, map[string]string{"podconfig": podConfig.ObjectMeta.Name})
			if err != nil {
				reqLogger.Error(err, "Failed to reconcile resource", "Name", "cnf-example", "Namespace", "cnf-test")
				return reconcile.Result{}, err
			}

		}

		// Get the list of pods that have a podconfig label
		podList := &corev1.PodList{}
		err = r.Client.List(context.TODO(), podList, client.MatchingLabels{"podconfig": podConfig.ObjectMeta.Name})
		if err != nil {
			fmt.Println(err)
		}
		// Pods need to be at least created to proceed
		// Checking if the list is empty
		if len(podList.Items) <= 0 {
			return reconcile.Result{}, nil
		}
		// Apply configuration defined in the podconfig CR to pods with the appropriate label.
		for _, pod := range podList.Items {

			// Pods need to be running in order to receive new configuration
			// Wait for pod phase running
			if pod.Status.Phase != "Running" {
				fmt.Printf("pod %v phase is %v, requeuing... ", pod.ObjectMeta.Name, pod.Status.Phase)
				return reconcile.Result{}, nil
			}

			applyConfig(pod, &podConfig)

		}
	}
	return reconcile.Result{}, nil
}

// SetupWithManager for the podconfig controller
func (r *PodConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&podconfigv1alpha1.PodConfig{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
