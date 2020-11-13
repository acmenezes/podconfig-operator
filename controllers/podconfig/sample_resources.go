package controllers

import (
	"context"
	"fmt"
	"time"

	podconfigv1alpha1 "github.com/acmenezes/podconfig-operator/apis/podconfig/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *PodConfigReconciler) createSampleDeployment(podConfig *podconfigv1alpha1.PodConfig, name string, namespace string, label map[string]string) error {

	deployment := &appsv1.Deployment{}

	objectMeta := setObjectMeta(name, namespace, label)

	err := r.reconcileResource(r.createDeploymentForPodConfig, podConfig, deployment, objectMeta)
	if err != nil {
		return err
	}
	return nil
}

func (r *PodConfigReconciler) createDeploymentForPodConfig(PodConfig *podconfigv1alpha1.PodConfig, objectMeta metav1.ObjectMeta) runtime.Object {

	var replicas int32 = 2
	deploy := &appsv1.Deployment{
		ObjectMeta: objectMeta,
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: objectMeta.Labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: objectMeta,
				Spec: corev1.PodSpec{
					ServiceAccountName: "default",
					Containers: []corev1.Container{{
						Name:  "cnf-example",
						Image: "nicolaka/netshoot:latest",
						// Image:           "registry.access.redhat.com/ubi8/ubi:latest",
						ImagePullPolicy: corev1.PullAlways,
						Command:         []string{"/bin/bash", "-c", "--"},
						Args:            []string{"while true; do sleep 30; done;"},
					}},
				},
			},
		},
	}
	// Set PodConfig instance as the owner and controller
	controllerutil.SetControllerReference(PodConfig, deploy, r.Scheme)
	return deploy
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
	objectMeta metav1.ObjectMeta) error {

	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: objectMeta.Name, Namespace: objectMeta.Namespace}, resource)
	fmt.Printf("%v - Getting sample resource error boolean: %v", time.Now(), errors.IsNotFound(err))
	if err != nil && errors.IsNotFound(err) {

		// Define a new resource

		fmt.Printf("%v - reconcileResource: creating a new sample resource for podconfigs", time.Now())

		resource := createResource(podconfig, objectMeta)
		err = r.Client.Create(context.TODO(), resource)

		if err != nil {
			fmt.Printf("%v - reconcileResource: failed to create a new sample resource err: %v", time.Now(), err)
			return err
		}

		// Resource created successfully - return and requeue
		return nil
	}

	fmt.Printf("%v - reconcileResource: Message getting the desired sample resource from API %v", time.Now(), err)
	return err
}
