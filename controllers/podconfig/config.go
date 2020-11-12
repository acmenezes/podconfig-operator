package controllers

import (
	"encoding/json"
	"fmt"
	"os/exec"

	podconfigv1alpha1 "github.com/acmenezes/podconfig-operator/apis/podconfig/v1alpha1"
	"github.com/containernetworking/plugins/pkg/ns"
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

	err = createNetworkAttachments(pod.ObjectMeta.Name, pid, podconfig.Spec.NetworkAttachments)
	if err != nil {
		fmt.Printf("Error creating network attachments: %v\n", err)
		return err
	}

	// err = newVLANsforPod(pod.ObjectMeta.Name, pid, podconfig)
	// if err != nil {
	// 	fmt.Println(err)
	// 	return err
	// }

	return nil
}

func createNetworkAttachments(podName string, pid string, networkAttachments []podconfigv1alpha1.Link) error {

	for _, na := range networkAttachments {
		// Create bridge in host namespace
		err := createBridge(na.Master)
		if err != nil {
			fmt.Printf("Error creating bridge device %s: %v\n", na.Master, err)
			return err
		}

		// Setting bridge IP
		addr, _ := netlink.ParseAddr("192.168.100.1/24")

		err = addBridgeIP(na.Master, addr)
		if err != nil {
			fmt.Printf("Error configuring IP address for bridge %s err: %v", na.Master, err)
			return err
		}

		// Setting bridge up
		err = setBridgeUp(na.Master)
		if err != nil {
			fmt.Printf("Error setting bridge %s up. err: %v", na.Master, err)
			return err
		}

		// err = newVethForPod(podName, pid, na.Name, na.Master)
		// if err != nil {
		// 	fmt.Printf("Error creating new veth pair for pod: %v\n", err)
		// 	return err
		// }
	}
	fmt.Println("New network attachment created successfully.")
	return nil
}

func createBridge(bridge string) error {

	targetNS, err := ns.GetNS("/tmp/proc/1/ns/net")

	if err != nil {
		fmt.Printf("Error getting host network namespace: %v\n", err)
	}

	err = targetNS.Do(func(hostNs ns.NetNS) error {
		br := &netlink.Bridge{
			LinkAttrs: netlink.LinkAttrs{
				Name: bridge,
			},
		}
		return netlink.LinkAdd(br)
	})
	fmt.Println("Bridge created successfully on the host.")
	return nil
}

func addBridgeIP(bridge string, ipAddr *netlink.Addr) error {

	targetNS, err := ns.GetNS("/tmp/proc/1/ns/net")

	if err != nil {
		fmt.Printf("Error getting host network namespace: %v\n", err)
	}

	err = targetNS.Do(func(hostNs ns.NetNS) error {
		br := &netlink.Bridge{
			LinkAttrs: netlink.LinkAttrs{
				Name: bridge,
			},
		}
		return netlink.AddrAdd(br, ipAddr)
	})
	fmt.Printf("Added ip %v to bridge %s ", ipAddr.IP, bridge)
	return nil
}

func setBridgeUp(bridge string) error {

	targetNS, err := ns.GetNS("/tmp/proc/1/ns/net")

	if err != nil {
		fmt.Printf("Error getting host network namespace: %v\n", err)
	}

	err = targetNS.Do(func(hostNs ns.NetNS) error {
		br := &netlink.Bridge{
			LinkAttrs: netlink.LinkAttrs{
				Name: bridge,
			},
		}
		return netlink.LinkSetUp(br)
	})
	fmt.Printf("Link up on bridge %s", bridge)
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
	fmt.Printf("Container PID is %v\n", pid)

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
		fmt.Printf("Error moving the veth peer to host: %v\n", stderr)
		return stderr
	}
	fmt.Printf("%s\n", stdout)
	fmt.Println("Host peer interface " + hostVeth + " moved to root namespace successfully!\n")

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
	fmt.Println("Host peer interface " + hostVeth + " up!\n")

	err := connectVethToBridge(hostVeth, bridgeName)
	if err != nil {
		fmt.Printf("Error connecting veth %s to bridge %s: %v\n", hostVeth, bridgeName, err)
		return err
	}

	return nil
}

func connectVethToBridge(hostVeth string, bridge string) error {

	// Connecting the host peer interface to the selected
	fmt.Println("Connecting interface " + hostVeth + " to bridge " + bridge + "\n")

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

	fmt.Println("Host peer interface " + hostVeth + " connected to bridge " + bridge + "\n")
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
		fmt.Println("New Vlan configuration for pod " + podName + ": " + string(vlanJSON) + "\n")
		fmt.Printf("Container PID is %v\n", pid)
	}

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
