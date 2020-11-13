package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	cri "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

func getCRIOConnection() (*grpc.ClientConn, error) {

	var conn *grpc.ClientConn

	conn, err := grpc.Dial("unix:///var/run/crio/crio.sock", grpc.WithInsecure())

	if err != nil {
		fmt.Println("Connection failed: ", err)
		return nil, err
	}
	fmt.Println("Connected with CRI-O at unix:///var/run/crio/crio.sock")

	return conn, nil
}

func getCRIOContainerStatus(containerID string, grpcConn *grpc.ClientConn) (*cri.ContainerStatusResponse, error) {

	criClient := cri.NewRuntimeServiceClient(grpcConn)

	request := &cri.ContainerStatusRequest{
		ContainerId: containerID,
		Verbose:     true,
	}
	response, err := cri.RuntimeServiceClient.ContainerStatus(criClient, context.Background(), request)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func parseCRIOContainerInfo(statusResponse *cri.ContainerStatusResponse) map[string]interface{} {

	var parsedContainerInfo map[string]interface{}

	containerInfo := statusResponse.Info["info"]

	json.Unmarshal([]byte(containerInfo), &parsedContainerInfo)

	return parsedContainerInfo
}

func getPid(parsedContainerInfo map[string]interface{}) string {

	return fmt.Sprintf("%.0f", parsedContainerInfo["pid"])

}

func getContainerIDs(pod corev1.Pod) []string {

	containerIDs := []string{}

	// get container ID list
	for _, containerStatus := range pod.Status.ContainerStatuses {

		containerIDs = append(containerIDs, containerStatus.ContainerID[8:])

	}
	return containerIDs
}
