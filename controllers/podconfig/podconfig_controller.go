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
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

	// TODO: list all podconfigs provided in the namespace
	// TODO: reconcile one by one checking the pods labelled with respective podconfigs

	// Fetch the PodConfig object
	podconfig := &podconfigv1alpha1.PodConfig{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, podconfig)

	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// TODO: Update the status field with conditions while creating the new instance

	// Deployments to mockup CNFs on OpenShift Cluster
	// Creating deployment with 2 replicas to simulate CNF-to-CNF communication over Linux VLANs

	deployment := &appsv1.Deployment{}

	objectMeta := setObjectMeta("cnf-example", "cnf-test", map[string]string{"podconfig": podconfig.Name})

	res, err := r.reconcileResource(r.createDeploymentForPodConfig, podconfig, deployment, objectMeta, reqLogger)
	if err != nil {
		reqLogger.Error(err, "Failed to reconcile resource", "Name", "cnf-example", "Namespace", "cnf-test")
		return reconcile.Result{}, err
	}

	// Get the list of pods that have a podconfig label and retrive their first container IDs
	podList := &corev1.PodList{}
	containerIDs := []string{}
	err = r.Client.List(context.TODO(), podList, client.MatchingLabels{"podconfig": podconfig.Name})
	if err == nil {
		for _, pod := range podList.Items {
			fmt.Printf("Pod Name: %s \n", pod.GetName())
			fmt.Printf("Container ID: %s\n", pod.Status.ContainerStatuses[0].ContainerID)
			containerIDs = append(containerIDs, pod.Status.ContainerStatuses[0].ContainerID[8:])
		}
	} else {
		fmt.Printf("unknown command\n")
	}

	// Connect with CRI-O's grpc endpoint
	conn, err := getCRIOConnection()
	if err != nil {
		fmt.Println(err)
		return res, err
	}

	// Make a container status request to CRI-O
	containerStatusResponseList, err := getCRIOContainerStatus(containerIDs, conn)
	if err != nil {
		fmt.Println(err)
		return res, nil
	}
	for _, resp := range containerStatusResponseList {

		var parsedContainerInfo map[string]interface{}

		containerInfo := resp.Info["info"]

		json.Unmarshal([]byte(containerInfo), &parsedContainerInfo)

		fmt.Println("Container pid is ", parsedContainerInfo["pid"])
	}

	// for _, id := range containerIDs {
	// 	fmt.Printf("!!!!!!!!!!!!!!!!!!!!   Container ID is: %s\n", id)
	// }
	// fmt.Println("Pods with configurations: %v", podList.Items[0])

	return res, nil
}

// SetupWithManager for the podconfig controller
func (r *PodConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&podconfigv1alpha1.PodConfig{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

func setObjectMeta(name string, namespace string, labels map[string]string) metav1.ObjectMeta {
	objectMeta := metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels:    labels,
	}
	return objectMeta
}

type createResourceFunc func(podconfig *podconfigv1alpha1.PodConfig, objectMeta metav1.ObjectMeta) runtime.Object

func (r *PodConfigReconciler) reconcileResource(
	createResource createResourceFunc,
	podconfig *podconfigv1alpha1.PodConfig,
	resource runtime.Object,
	objectMeta metav1.ObjectMeta,
	reqLogger logr.Logger) (ctrl.Result, error) {

	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: objectMeta.Name, Namespace: objectMeta.Namespace}, resource)

	if err != nil && errors.IsNotFound(err) {

		// Define a new resource
		resource := createResource(podconfig, objectMeta)
		err = r.Client.Create(context.TODO(), resource)

		if err != nil {
			return reconcile.Result{}, err
		}

		// Resource created successfully - return and requeue
		return reconcile.Result{Requeue: true}, nil

	} else if err != nil {

		return reconcile.Result{}, err

	}

	return ctrl.Result{}, nil
}
