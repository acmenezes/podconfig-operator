# podconfig-operator
The Runtime Configuration Operator for Unprivileged Pods Running on OpenShift platform

Some times pods or pod owners may not have the system privileges to do dynamic special configurations on their workloads. An example is CNF that needs to change interface configurations on the fly to create new connections but running in a totally restricted pod with no privilges or network related Linux capabilities. For that an operator could be a good solution once it's a trusted application controlled by OLM.
