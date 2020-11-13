package controllers

import (
	"fmt"

	podconfigv1alpha1 "github.com/acmenezes/podconfig-operator/apis/podconfig/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func applyConfig(pod corev1.Pod, podconfig *podconfigv1alpha1.PodConfig) error {

	// Get the container IDs for the given pod
	containerIDs := getContainerIDs(pod)

	// Connect with CRI-O's grpc endpoint
	conn, err := getCRIOConnection()
	if err != nil {
		fmt.Printf("Error getting CRIO connection: %v\n", err)
		return err
	}

	// Make a container status request to CRI-O
	// Here it doesn't matter which container ID inside the pod.
	// The goal is to put runtime configurations on Pod shared namespaces
	// like network and mount. Not intended for process/container specific namespaces.

	containerStatusResponse, err := getCRIOContainerStatus(containerIDs[0], conn)
	if err != nil {
		fmt.Printf("Error getting CRIO container status: %v\n", err)
		return err
	}

	// Parse response and get the first container pid
	pid := getPid(parseCRIOContainerInfo(containerStatusResponse))

	err = createNetworkAttachments(pod.ObjectMeta.Name, pid, podconfig.Spec.NetworkAttachments)
	if err != nil {
		fmt.Printf("Error creating network attachments: %v\n", err)
		return err
	}

	return nil
}

func createNetworkAttachments(podName string, pid string, networkAttachments []podconfigv1alpha1.Link) error {

	for _, na := range networkAttachments {

		err := getBridgeOnHost(na.Master)

		if err != nil {

			fmt.Printf("%v\n", err)
			fmt.Println("Creating bridge on Host.")

			// Create bridge in host namespace
			err := createBridge(na.Master, ips.getFreeIP(na.CIDR))
			if err != nil {
				fmt.Printf("Error creating bridge device %s: %v\n", na.Master, err)
				return err
			}

		}

		// Create veth pairs for the new networkAttachment
		err = newVethForPod(pid, na)
		if err != nil {
			fmt.Printf("Error creating new veth pair for pod: %v\n", err)
			return err
		}
	}

	fmt.Println("New network attachment created successfully.")
	return nil
}
