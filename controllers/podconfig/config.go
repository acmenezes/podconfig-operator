package controllers

import (
	"encoding/json"
	"fmt"
	"os/exec"

	podconfigv1alpha1 "github.com/acmenezes/podconfig-operator/apis/podconfig/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func applyConfig(pod corev1.Pod, podconfig *podconfigv1alpha1.PodConfig) error {

	// Get the container IDs for the given pod
	containerIDs := getContainerIDs(pod)

	// Connect with CRI-O's grpc endpoint
	conn, err := getCRIOConnection()
	if err != nil {
		fmt.Println(err)
		return err
	}

	// Make a container status request to CRI-O
	// Here it doesn't matter which container ID inside the pod.
	// The goal is to put runtime configurations on Pod shared namespaces
	// like network and mount. Not intended for process/container specific namespaces.

	containerStatusResponse, err := getCRIOContainerStatus(containerIDs[0], conn)
	if err != nil {
		fmt.Println(err)
		return err
	}

	// Parse response and get the first container pid
	pid := getPid(parseCRIOContainerInfo(containerStatusResponse))

	newVLANsforPod(pod.ObjectMeta.Name, pid, podconfig)

	return nil
}

func newVLANsforPod(podName string, pid string, podconfig *podconfigv1alpha1.PodConfig) {

	for _, vlan := range podconfig.Spec.Vlans {
		stdout, stderr := exec.Command("nsenter", "-t", pid,
			"--net",
			"ip", "link", "add",
			"link", vlan.ParentInterfaceName,
			"name", vlan.ParentInterfaceName+"."+fmt.Sprintf("%v", vlan.VlanID),
			"type", "vlan",
			"id", fmt.Sprintf("%v", vlan.VlanID)).Output()

		if stderr != nil {
			fmt.Printf("%s\n", stdout)
			fmt.Println(stderr)
		} else {
			vlanJSON, err := json.Marshal(vlan)
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Printf("%s\n", stdout)
			fmt.Println("New Vlan configuration for pod " + podName + ": " + string(vlanJSON))
		}
	}
}

// func newBridge()
// func newEthInterface()
// func newVXLanInterface()
// func ipRoute()
