# podconfig-operator
The Runtime Configuration Operator for Unprivileged Pods Running on OpenShift platform

Some times pods or pod owners may not have the system privileges to do dynamic special configurations on their workloads. An example is a CNF that needs to change interface configurations on the fly to create new connections but running in a totally restricted pod with no privilges or network related Linux capabilities. How could a CNF apply those configurations like injecting a new VLAN subinterface and connecting to a switching or bridging virtual device with no CAP_NET_ADMIN on the user? 

On those cases an operator could be a good solution. If the operator is a trusted application and open source the community could leverage it's capabilities to change configurations dynamically on the Pods in domains that the regular Pod abstraction can't. One of them is the network domain that is  currently owned by the CNI plugin in use by the cluster and configured only once at the pod's creation/initialization time.
