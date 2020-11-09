package controllers

import (
	"fmt"
	"os/exec"
)

func applyConfig(containerID string) error {

	// Connect with CRI-O's grpc endpoint
	conn, err := getCRIOConnection()
	if err != nil {
		fmt.Println(err)
		return err
	}

	// Make a container status request to CRI-O
	containerStatusResponse, err := getCRIOContainerStatus(containerID, conn)
	if err != nil {
		fmt.Println(err)
		return err
	}

	// Parse response and get container pid for namespace
	pid := getPid(parseCRIOContainerInfo(containerStatusResponse))

	newVLANforPod(pid)

	return nil
}

func newVLANforPod(pid string) {

	cmd := exec.Command("nsenter", "-t", pid, "--net", "ip", "link", "add", "link", "eth0", "name", "eth0.8", "type", "vlan", "id", "8")
	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("!!!!! Interface configured with success !!!!!!")
	}
}

// 6 - Set the vlan interface https://gist.github.com/milosgajdos/9f68b1818dca886e9ae8

// 7 - Set the bridge
