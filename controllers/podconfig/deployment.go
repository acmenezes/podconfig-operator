package controllers

import (
	PodConfig "github.com/acmenezes/podconfig-operator/apis/podconfig/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *PodConfigReconciler) deploymentForPodConfig(PodConfig *PodConfig.PodConfig) *appsv1.Deployment {

	var replicas int32 = 2
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cnf-example",
			Namespace: "cnf-test",
			Labels:    map[string]string{"app": "podconfig-operator"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "podconfig-operator"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "podconfig-operator"},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "default",
					Containers: []corev1.Container{{
						Name:            "cnf-example",
						Image:           "registry.access.redhat.com/ubi8/ubi:latest",
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
