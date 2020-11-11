package controllers

import (
	"encoding/json"
	"fmt"
	"os/exec"

	podconfigv1alpha1 "github.com/acmenezes/podconfig-operator/apis/podconfig/v1alpha1"
	"github.com/vishvananda/netlink"
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

	// Create bridge in host namespace
	err = createBridge(1, podconfig.Spec.Bridge)
	if err != nil {
		fmt.Printf("Error creating bridge device %s: %v\n", podconfig.Spec.Bridge, err)
		return err
	}

	err = newVethForPod(pod.ObjectMeta.Name, pid, podconfig.Spec.Veth, podconfig.Spec.Bridge)
	if err != nil {
		fmt.Printf("Error creating new veth pair for pod: %v\n", err)
		return err
	}

	// err = newVLANsforPod(pod.ObjectMeta.Name, pid, podconfig)
	// if err != nil {
	// 	fmt.Println(err)
	// 	return err
	// }

	return nil
}

// Get the vlan listed on the pod configuration and create subinterfaces for the given Pod.
func newVLANsforPod(podName string, pid string, podconfig *podconfigv1alpha1.PodConfig) error {

	for _, vlan := range podconfig.Spec.Vlans {

		// Creates a new Veth pair to allocate the vlans as subinterfaces
		// This example works with only with new Veth pairs
		// TODO: future work is check the existence of each Veth interface
		err := newVethForPod(podName, pid, vlan.ParentInterfaceName, vlan.BridgeName)
		if err != nil {
			fmt.Printf("Error creating new veth pair for pod: %v\n", err)
			return err
		}

		// Combination of 2 commands nsenter with -t (target) and container process ID
		// from pid it retrieves the the namespace fd (file descriptor) with host /proc mounted
		// Then it executes a series of ip command options to configure the network
		stdout, stderr := exec.Command("nsenter", "-t", pid,
			"--net",
			"ip", "link", "add",
			"link", vlan.ParentInterfaceName,
			"name", vlan.ParentInterfaceName+"."+fmt.Sprintf("%v", vlan.VlanID),
			"type", "vlan",
			"id", fmt.Sprintf("%v", vlan.VlanID)).Output()

		if stderr != nil {
			fmt.Printf("%s\n", stdout)
			fmt.Printf("Error running ip link add for vlans: %v\n", stderr)
			return err
		}
		vlanJSON, err := json.Marshal(vlan)
		if err != nil {
			fmt.Printf("Error marshaling vlans to json: %v\n", err)
			return err
		}
		fmt.Printf("%s\n", stdout)
		fmt.Println("New Vlan configuration for pod " + podName + ": " + string(vlanJSON))
		fmt.Printf("Container PID is %v", pid)
	}

	return nil
}

// Creates the veth pair of interfaces in the container and sets up both
// with bridge and namespaces
func newVethForPod(podName string, pid string, podVeth string, bridgeName string) error {

	// Creating the veth pair in the Pod's network namespace

	// Combination of 2 commands nsenter with -t (target) and container process ID
	// from pid it retrieves the the namespace fd (file descriptor) with host /proc mounted
	// Then it executes a series of ip command options to configure the network

	hostVeth := "h" + podVeth + pid
	podVeth = podVeth + pid

	stdout, stderr := exec.Command("nsenter", "-t", pid,
		"--net",
		"ip", "link", "add",
		"name", hostVeth,
		"type", "veth",
		"peer", "name", podVeth).Output()
	if stderr != nil {
		fmt.Printf("%s\n", stdout)
		fmt.Printf("Error running ip link add for veth pair: %v\n", stderr)
		return stderr
	}
	fmt.Printf("%s\n", stdout)
	fmt.Printf("Container PID is %v", pid)

	// Taking the host interface to the host namespace
	// pointing pid 1 for host namespace (/proc in container is the host's)

	// Combination of 2 commands nsenter with -t (target) and container process ID
	// from pid it retrieves the the namespace fd (file descriptor) with host /proc mounted
	// Then it executes a series of ip command options to configure the network

	fmt.Println("Taking the host peer interface " + hostVeth + " to the host namespace...")
	stdout, stderr = exec.Command("nsenter", "-t", pid,
		"--net",
		"ip", "link", "set", hostVeth, "netns", "1").Output()
	if stderr != nil {
		fmt.Printf("%s\n", stdout)
		fmt.Printf("%v\n", stderr)
		return stderr
	}
	fmt.Printf("%s\n", stdout)
	fmt.Println("Host peer interface " + hostVeth + " moved to root namespace successfully!")

	// setting the host peer interface up

	// Combination of 2 commands nsenter with -t (target) and container process ID
	// from pid it retrieves the the namespace fd (file descriptor) with host /proc mounted
	// Then it executes a series of ip command options to configure the network

	fmt.Println("Setting interface " + hostVeth + " up...")

	stdout, stderr = exec.Command("nsenter", "-t", "1",
		"--net",
		"ip", "link", "set", hostVeth, "up").Output()
	if stderr != nil {
		fmt.Printf("%s\n", stdout)
		fmt.Printf("Error running ip link set veth up: %v\n", stderr)
		return stderr
	}

	fmt.Printf("%s\n", stdout)
	fmt.Println("Host peer interface " + hostVeth + " up!")

	err := connectVethToBridge(hostVeth, bridgeName)
	if err != nil {
		fmt.Printf("Error connecting veth %s to bridge %s: %v\n", hostVeth, bridgeName, err)
		return err
	}

	return nil
}

func connectVethToBridge(hostVeth string, bridge string) error {

	// Connecting the host peer interface to the selected
	fmt.Println("Connecting interface " + hostVeth + " to bridge " + bridge)

	veth, err := netlink.LinkByName(hostVeth)
	if err != nil {
		fmt.Printf("Error getting the link for %s: %v\n", hostVeth, err)
		return err
	}

	linkAttrs := netlink.NewLinkAttrs()
	linkAttrs.Name = bridge
	br := &netlink.Bridge{LinkAttrs: linkAttrs}

	err = netlink.LinkSetMaster(veth, br)
	if err != nil {
		fmt.Printf("Error setting master device to %s: %v\n", veth, err)
		return err
	}

	fmt.Println("Host peer interface " + hostVeth + " connected to bridge " + bridge)
	return nil
}

// Creates a bridge and sets it on the Namespace for pid
func createBridge(pid int, bridge string) error {

	linkAttrs := netlink.NewLinkAttrs()
	linkAttrs.Name = bridge
	br := &netlink.Bridge{LinkAttrs: linkAttrs}
	err := netlink.LinkAdd(br)

	if err != nil {
		fmt.Printf("could not add %s: %v\n", linkAttrs.Name, err)
	}

	err = netlink.LinkSetNsPid(br, pid)
	if err != nil {
		fmt.Printf("Error moving bridge to selected namespace: %v\n", err)
	}

	fmt.Printf("Bridge %s created successfully", linkAttrs.Name)

	return nil
}

// Testing with the netlink library

// func newVEthInterfaceForPod(podName string, pid string, ifName string) error {
// 	hostVeth := "h" + ifName

// 	podVeth := &netlink.Veth{
// 		LinkAttrs: netlink.LinkAttrs{
// 			Name:      ifName,
// 			Namespace: pid,
// 		},
// 		PeerName: hostVeth,
// 	}

// 	if err := netlink.LinkAdd(podVeth); err != nil {
// 		fmt.Printf("Error adding veth %+v: %s", podVeth, err)
// 		return err
// 	}

// 	fmt.Println("Interface " + ifName + " configured successfully")
// 	fmt.Printf("Container PID is %v", pid)
// 	return nil
// }

// func newBridge()
// func newEthInterface()
// func newVXLanInterface()
// func ipRoute()
