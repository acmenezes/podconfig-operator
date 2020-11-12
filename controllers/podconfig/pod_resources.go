package controllers

import (
	PodConfig "github.com/acmenezes/podconfig-operator/apis/podconfig/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *PodConfigReconciler) createDeploymentForPodConfig(PodConfig *PodConfig.PodConfig, objectMeta metav1.ObjectMeta) runtime.Object {

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
