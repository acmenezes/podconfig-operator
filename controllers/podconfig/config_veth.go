package controllers

import (
	"fmt"

	podconfigv1alpha1 "github.com/acmenezes/podconfig-operator/apis/podconfig/v1alpha1"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

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

		// Attempt to check the existence of the pod veth
		// If if already exists it skips creation and configuration
		// If any other error comes up it attempts to create
		// TODO: check netlink error types to better handle this

		// _, err := netlink.LinkByName(podVethName)
		// if err != nil {

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
		// }
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
