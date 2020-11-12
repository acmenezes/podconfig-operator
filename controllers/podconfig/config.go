package controllers

import (
	"fmt"

	podconfigv1alpha1 "github.com/acmenezes/podconfig-operator/apis/podconfig/v1alpha1"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
	corev1 "k8s.io/api/core/v1"
)

var ipsInUse []string

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
		addr, _ := netlink.ParseAddr(fmt.Sprintf("192.168.100.1/24"))
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

		err = newVethForPod(pid, na)

		if err != nil {
			fmt.Printf("Error creating new veth pair for pod: %v\n", err)
			return err
		}
	}
	fmt.Println("New network attachment created successfully.")
	return nil
}

func getFreeIP() *netlink.Addr {

	ipInUse := false
	var addr *netlink.Addr

	for i := 2; i <= 254; i++ {
		for _, ip := range ipsInUse {
			if ip == fmt.Sprintf("192.168.100.%d/24", i) {
				ipInUse = true
			}
		}

		if ipInUse == true {
			ipInUse = false
			continue
		} else {
			addr, _ = netlink.ParseAddr(fmt.Sprintf("192.168.100.%d/24", i))
			ipsInUse = append(ipsInUse, fmt.Sprintf("192.168.100.%d/24", i))
			break
		}

	}
	return addr
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

func newVethForPod(pid string, networkAttachment podconfigv1alpha1.Link) error {

	// Get the pods namespace object
	targetNS, err := ns.GetNS("/tmp/proc/" + pid + "/ns/net")

	if err != nil {
		return fmt.Errorf("Error getting Pod network namespace: %v", err)
	}

	// Appending the process id number to the names to identify the links
	// with the container processes

	podVethName := networkAttachment.Name + pid
	hostVethName := "h" + networkAttachment.Name + pid

	// The Do function takes cares of all side effects of switching namespaces
	// and spawning new threads or child processes on the destination namespaces
	// Since targetNS belongs to pod all instructions enclosed by Do() will be run
	// on the pods namespace

	err = targetNS.Do(func(hostNs ns.NetNS) error {
		veth := &netlink.Veth{
			LinkAttrs: netlink.LinkAttrs{
				Name: podVethName,
			},
			PeerName: hostVethName,
		}
		err := netlink.LinkAdd(veth)
		if err != nil {
			return fmt.Errorf("failed to set %q up: %w", podVethName, err)
		}

		// Get newly created pod link by name
		podVeth, err := netlink.LinkByName(podVethName)

		if err != nil {
			return fmt.Errorf("failed to lookup %q: %v", podVethName, err)
		}

		// Add ip address to pod veth
		err = netlink.AddrAdd(podVeth, getFreeIP())
		if err != nil {
			return fmt.Errorf("failed to add IP addr to %q: %v", podVeth, err)
		}

		// Set pod veth link up
		err = netlink.LinkSetUp(podVeth)
		if err != nil {
			return fmt.Errorf("failed to set %q up: %w", podVethName, err)
		}

		// Move host end of the link to the host and continue
		// the configuration from the host network namespace

		targetNS, err := ns.GetNS("/tmp/proc/1/ns/net")

		hostVeth, _ := netlink.LinkByName(hostVethName)

		err = netlink.LinkSetNsFd(hostVeth, int(targetNS.Fd()))
		if err != nil {
			return fmt.Errorf("failed to move veth to host netns: %v", err)
		}

		return nil
	})
	if err != nil {
		fmt.Printf("%v\n", err)
	}

	targetNS, err = ns.GetNS("/tmp/proc/1/ns/net")

	err = targetNS.Do(func(hostNs ns.NetNS) error {

		// Get host veth link by name
		hostVeth, err := netlink.LinkByName(hostVethName)
		if err != nil {
			return fmt.Errorf("failed to lookup %q: %v", hostVethName, err)
		}

		// // Add ip address to host veth link
		// addr, _ := netlink.ParseAddr("192.168.100.3/24")

		// err = netlink.AddrAdd(hostVeth, addr)
		// if err != nil {
		// 	return fmt.Errorf("failed to add IP addr to %q: %v", hostVeth, err)
		// }

		// Set host veth link up
		if err = netlink.LinkSetUp(hostVeth); err != nil {
			return fmt.Errorf("failed to set %q up: %w", hostVethName, err)
		}

		// Set host veth link master bridge
		br, err := netlink.LinkByName(networkAttachment.Master)
		err = netlink.LinkSetMaster(hostVeth, br)
		if err != nil {
			return fmt.Errorf("Error setting master device to %s: %v", hostVethName, err)
		}

		return nil
	})

	if err != nil {
		fmt.Printf("%v\n", err)
	}

	fmt.Println("Veth pair created successfully")
	return nil
}

// Creates the veth pair of interfaces in the container and sets up both
// with bridge and namespaces
// func newVethForPod(podName string, pid string, podVeth string, bridgeName string) error {

// 	// Creating the veth pair in the Pod's network namespace

// 	// Combination of 2 commands nsenter with -t (target) and container process ID
// 	// from pid it retrieves the the namespace fd (file descriptor) with host /proc mounted
// 	// Then it executes a series of ip command options to configure the network

// 	hostVeth := "h" + podVeth + pid
// 	podVeth = podVeth + pid

// 	stdout, stderr := exec.Command("nsenter", "-t", pid,
// 		"--net",
// 		"ip", "link", "add",
// 		"name", hostVeth,
// 		"type", "veth",
// 		"peer", "name", podVeth).Output()
// 	if stderr != nil {
// 		fmt.Printf("%s\n", stdout)
// 		fmt.Printf("Error running ip link add for veth pair: %v\n", stderr)
// 		return stderr
// 	}
// 	fmt.Printf("%s\n", stdout)
// 	fmt.Printf("Container PID is %v\n", pid)

// 	// Taking the host interface to the host namespace
// 	// pointing pid 1 for host namespace (/proc in container is the host's)

// 	// Combination of 2 commands nsenter with -t (target) and container process ID
// 	// from pid it retrieves the the namespace fd (file descriptor) with host /proc mounted
// 	// Then it executes a series of ip command options to configure the network

// 	fmt.Println("Taking the host peer interface " + hostVeth + " to the host namespace...")
// 	stdout, stderr = exec.Command("nsenter", "-t", pid,
// 		"--net",
// 		"ip", "link", "set", hostVeth, "netns", "1").Output()
// 	if stderr != nil {
// 		fmt.Printf("%s\n", stdout)
// 		fmt.Printf("Error moving the veth peer to host: %v\n", stderr)
// 		return stderr
// 	}
// 	fmt.Printf("%s\n", stdout)
// 	fmt.Println("Host peer interface " + hostVeth + " moved to root namespace successfully!\n")

// 	// setting the host peer interface up

// 	// Combination of 2 commands nsenter with -t (target) and container process ID
// 	// from pid it retrieves the the namespace fd (file descriptor) with host /proc mounted
// 	// Then it executes a series of ip command options to configure the network

// 	fmt.Println("Setting interface " + hostVeth + " up...")

// 	stdout, stderr = exec.Command("nsenter", "-t", "1",
// 		"--net",
// 		"ip", "link", "set", hostVeth, "up").Output()
// 	if stderr != nil {
// 		fmt.Printf("%s\n", stdout)
// 		fmt.Printf("Error running ip link set veth up: %v\n", stderr)
// 		return stderr
// 	}

// 	fmt.Printf("%s\n", stdout)
// 	fmt.Println("Host peer interface " + hostVeth + " up!\n")

// 	err := connectVethToBridge(hostVeth, bridgeName)
// 	if err != nil {
// 		fmt.Printf("Error connecting veth %s to bridge %s: %v\n", hostVeth, bridgeName, err)
// 		return err
// 	}

// 	return nil
// }

// Get the vlan listed on the pod configuration and create subinterfaces for the given Pod.
// func newVLANsforPod(podName string, pid string, podconfig *podconfigv1alpha1.PodConfig) error {

// 	for _, vlan := range podconfig.Spec.Vlans {

// 		// Creates a new Veth pair to allocate the vlans as subinterfaces
// 		// This example works with only with new Veth pairs
// 		// TODO: future work is check the existence of each Veth interface
// 		err := newVethForPod(podName, pid, vlan.ParentInterfaceName, vlan.BridgeName)
// 		if err != nil {
// 			fmt.Printf("Error creating new veth pair for pod: %v\n", err)
// 			return err
// 		}

// 		// Combination of 2 commands nsenter with -t (target) and container process ID
// 		// from pid it retrieves the the namespace fd (file descriptor) with host /proc mounted
// 		// Then it executes a series of ip command options to configure the network
// 		stdout, stderr := exec.Command("nsenter", "-t", pid,
// 			"--net",
// 			"ip", "link", "add",
// 			"link", vlan.ParentInterfaceName,
// 			"name", vlan.ParentInterfaceName+"."+fmt.Sprintf("%v", vlan.VlanID),
// 			"type", "vlan",
// 			"id", fmt.Sprintf("%v", vlan.VlanID)).Output()

// 		if stderr != nil {
// 			fmt.Printf("%s\n", stdout)
// 			fmt.Printf("Error running ip link add for vlans: %v\n", stderr)
// 			return err
// 		}
// 		vlanJSON, err := json.Marshal(vlan)
// 		if err != nil {
// 			fmt.Printf("Error marshaling vlans to json: %v\n", err)
// 			return err
// 		}
// 		fmt.Printf("%s\n", stdout)
// 		fmt.Println("New Vlan configuration for pod " + podName + ": " + string(vlanJSON) + "\n")
// 		fmt.Printf("Container PID is %v\n", pid)
// 	}

// 	return nil
// }

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
