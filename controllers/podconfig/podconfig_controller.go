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
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
	reqLogger := r.Log.WithValues("podconfig", req.NamespacedName)

	// Fetch the PodConfig instance
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

	deploy := &appsv1.Deployment{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "cnf-example", Namespace: "cnf-test"}, deploy)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		deploy := r.deploymentForPodConfig(podconfig)
		reqLogger.Info("Creating a new Deployment", "Deployment.Namespace", deploy.Namespace, "Deployment.Name", deploy.Name)
		err = r.Client.Create(context.TODO(), deploy)
		time.Sleep(500 * time.Millisecond)
		if err != nil {
			reqLogger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", deploy.Namespace, "Deployment.Name", deploy.Name)
			return reconcile.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Deployment")
		return reconcile.Result{}, err
	}

	// 1 - TODO: get the container IDs using the pod object itself under the deployment (it's on template Spec)

	// 2 - TODO: Connect with CRI-O through runtime-endpoint: unix:///var/run/crio/crio.sock
	// List of libraries to import:
	// "google.golang.org/grpc"
	// "k8s.io/kubernetes/pkg/kubelet/cri/remote"
	// "k8s.io/kubernetes/pkg/kubelet/util"
	//  Consider the get connection function on the bottom of the file

	// 3 - TODO: get the container status (a.k.a inspect) from cri api filtering with the IDs on step 1

	return ctrl.Result{}, nil
}

// SetupWithManager for the podconfig controller
func (r *PodConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&podconfigv1alpha1.PodConfig{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

// func getConnection(endPoints []string) (*grpc.ClientConn, error) {
// 	if endPoints == nil || len(endPoints) == 0 {
// 		return nil, fmt.Errorf("endpoint is not set")
// 	}
// 	endPointsLen := len(endPoints)
// 	var conn *grpc.ClientConn
// 	for indx, endPoint := range endPoints {
// 		logrus.Debugf("connect using endpoint '%s' with '%s' timeout", endPoint, Timeout)
// 		addr, dialer, err := util.GetAddressAndDialer(endPoint)
// 		if err != nil {
// 			if indx == endPointsLen-1 {
// 				return nil, err
// 			}
// 			logrus.Error(err)
// 			continue
// 		}
// 		conn, err = grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(Timeout), grpc.WithContextDialer(dialer))
// 		if err != nil {
// 			errMsg := errors.Wrapf(err, "connect endpoint '%s', make sure you are running as root and the endpoint has been started", endPoint)
// 			if indx == endPointsLen-1 {
// 				return nil, errMsg
// 			}
// 			logrus.Error(errMsg)
// 		} else {
// 			logrus.Debugf("connected successfully using endpoint: %s", endPoint)
// 			break
// 		}
// 	}
// 	return conn, nil
// }

// ContainerStatus sends a ContainerStatusRequest to the server, and parses
// the returned ContainerStatusResponse.
// func ContainerStatus(client pb.RuntimeServiceClient, ID, output string, tmplStr string, quiet bool) error {
// 	verbose := !(quiet)
// 	if output == "" { // default to json output
// 		output = "json"
// 	}
// 	if ID == "" {
// 		return fmt.Errorf("ID cannot be empty")
// 	}
// 	request := &pb.ContainerStatusRequest{
// 		ContainerId: ID,
// 		Verbose:     verbose,
// 	}
// 	logrus.Debugf("ContainerStatusRequest: %v", request)
// 	r, err := client.ContainerStatus(context.Background(), request)
// 	logrus.Debugf("ContainerStatusResponse: %v", r)
// 	if err != nil {
// 		return err
// 	}

// 	status, err := marshalContainerStatus(r.Status)
// 	if err != nil {
// 		return err
// 	}

// 	switch output {
// 	case "json", "yaml", "go-template":
// 		return outputStatusInfo(status, r.Info, output, tmplStr)
