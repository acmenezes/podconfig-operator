package controllers

import (
	"fmt"

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

// Bridge creation logic
// TODO: verify existence first and only creates if doesn't exist
// TODO: do the clean up on podconfig deletion - probable use for a finalizer
func getBridgeOnHost(bridge string) error {

	targetNS, err := ns.GetNS("/tmp/proc/1/ns/net")
	if err != nil {
		return fmt.Errorf("error getting host network namespace: %v", err)
	}

	err = targetNS.Do(func(hostNs ns.NetNS) error {

		_, err := netlink.LinkByName(bridge)
		if err != nil {
			return fmt.Errorf("error looking up for bridge %v %v", bridge, err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}
func createBridge(bridge string, ipAddr *netlink.Addr) error {

	targetNS, err := ns.GetNS("/tmp/proc/1/ns/net")
	if err != nil {
		return fmt.Errorf("error getting host network namespace: %v", err)
	}

	err = targetNS.Do(func(hostNs ns.NetNS) error {
		br := &netlink.Bridge{
			LinkAttrs: netlink.LinkAttrs{
				Name: bridge,
			},
		}
		// Creating bridge
		err := netlink.LinkAdd(br)
		if err != nil {
			return fmt.Errorf("failed to create bridge %v: %v", bridge, err)
		}

		// Setting bridge ip address
		err = netlink.AddrAdd(br, ipAddr)
		if err != nil {
			return fmt.Errorf("failed to set bridge ip address: %v", err)
		}

		// Setting bridge up
		err = netlink.LinkSetUp(br)
		if err != nil {
			return fmt.Errorf("failed to set bridge up: %v", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Println("Bridge created successfully on the host.")
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
		err = netlink.AddrAdd(podVeth, ips.getFreeIP(networkAttachment.CIDR))
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

		// Set host veth link up ( for PoC purposes it's only layer 2 on bridge)
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
