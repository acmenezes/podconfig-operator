package controllers

// 2 - TODO: Connect with CRI-O through runtime-endpoint: unix:///var/run/crio/crio.sock
// List of libraries to import:
// "google.golang.org/grpc"
// "k8s.io/kubernetes/pkg/kubelet/cri/remote"
// "k8s.io/kubernetes/pkg/kubelet/util"
//  Consider the get connection commented function on the bottom of the file

// 3 - TODO: get the container status (a.k.a inspect) from cri api filtering with the IDs on step 1
// It's necessary to set a RuntimeServiceClient and run containerStatus with context and the request

// 4 - Unmarshall the JSON output and get the pid of the container

// func getConnection(endPoints []string) (*grpc.ClientConn, error) {
// 	if endPoints == nil || len(endPoints) == 0 {
// 		return nil, fmt.Errorf("endpoint is not set")
// 	}
// 	endPointsLen := len(endPoints)
// 	var conn *grpc.ClientConn
// 	for indx, endPoint := range endPoints {
// 		logrus.Debugf("connect using endpoint '%s' with '%s' timeout", endPoint, Timeout)
// 		addr, dialer, err := util.GetAddressAndDialer(endPoint)
// 		if err != nil {
// 			if indx == endPointsLen-1 {
// 				return nil, err
// 			}
// 			logrus.Error(err)
// 			continue
// 		}
// 		conn, err = grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(Timeout), grpc.WithContextDialer(dialer))
// 		if err != nil {
// 			errMsg := errors.Wrapf(err, "connect endpoint '%s', make sure you are running as root and the endpoint has been started", endPoint)
// 			if indx == endPointsLen-1 {
// 				return nil, errMsg
// 			}
// 			logrus.Error(errMsg)
// 		} else {
// 			logrus.Debugf("connected successfully using endpoint: %s", endPoint)
// 			break
// 		}
// 	}
// 	return conn, nil
// }

// ContainerStatus sends a ContainerStatusRequest to the server, and parses
// the returned ContainerStatusResponse.
// func ContainerStatus(client pb.RuntimeServiceClient, ID, output string, tmplStr string, quiet bool) error {
// 	verbose := !(quiet)
// 	if output == "" { // default to json output
// 		output = "json"
// 	}
// 	if ID == "" {
// 		return fmt.Errorf("ID cannot be empty")
// 	}
// 	request := &pb.ContainerStatusRequest{
// 		ContainerId: ID,
// 		Verbose:     verbose,
// 	}
// 	logrus.Debugf("ContainerStatusRequest: %v", request)
// 	r, err := client.ContainerStatus(context.Background(), request)
// 	logrus.Debugf("ContainerStatusResponse: %v", r)
// 	if err != nil {
// 		return err
// 	}

// 	status, err := marshalContainerStatus(r.Status)
// 	if err != nil {
// 		return err
// 	}

// 	switch output {
// 	case "json", "yaml", "go-template":
// 		return outputStatusInfo(status, r.Info, output, tmplStr)
