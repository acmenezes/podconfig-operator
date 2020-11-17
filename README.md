# podconfig-operator

#### The Runtime Configuration Operator for Unprivileged Pods Running on Kubernetes

Some times pods or pod owners may not have the system privileges to do dynamic special configurations for their workloads. An example is a CNF that needs to change interface configurations on the fly to create new connections but running in a totally restricted pod with no privilges or network related Linux capabilities. How could a CNF apply those configurations without the root user on the container?

Tasks like injecting a VLAN subinterface, or simply creating a new veth pair connecting to a new selected device, getting a tunnel up on demand to connect to a service out of the cluster, setup different networking data planes that can't be managed by the CNI plugins and many others at runtime not only pod creation time require high privileges and may disrupt kubernetes elements that weren't intended to work like that.

On those cases an operator is the best solution. If the operator is a trusted application and open source the community could leverage it's capabilities to change configurations dynamically on Pods in domains that the regular Pod abstraction can't. One of them is the network domain that is  currently owned by the CNI plugin in use by the cluster and configured only once at the pod's creation/initialization time.

Some other highly privileged tasks like running temporary privileged binaries on behalf of unprivileged pods or setting up special deployments for long running tasks such as executing system tracing or eBPF tracing routines on containers can be a good fit for an operator like that when the target pod owners haven't or shouldn't have permissions to do that. Those features can be expanded as needed to other domains and be set as a configuration on a podConfig abstraction.

To know more about that check the [Design Proposal](docs/design_proposal.md) section.

---
#### Disclaimer
> This is an in progress development. This operator is in its early stages and should not be used in production in any hypothesis at this point.

---

### Install

**Note**
> Tested on OpenShift 4.5 and 4.6 only

First clone the project to the desired path.

```
git clone git@github.com:acmenezes/podconfig-operator.git
```

Then run the make deploy target all the necessary manifests will be applied.

```
make deploy
```
You should see something like this:
```
/usr/local/bin/controller-gen "crd:trivialVersions=true" rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
cd config/manager && /usr/local/bin/kustomize edit set image controller=quay.io/acmenezes/podconfig-operator:0.0.1
/usr/local/bin/kustomize build config/default | kubectl apply -f -
namespace/cnf-test created
customresourcedefinition.apiextensions.k8s.io/podconfigs.podconfig.opdev.io created
serviceaccount/podconfig-operator-sa created
role.rbac.authorization.k8s.io/leader-election-role created
role.rbac.authorization.k8s.io/manager-role created
role.rbac.authorization.k8s.io/role-scc-privileged created
clusterrole.rbac.authorization.k8s.io/manager-role created
rolebinding.rbac.authorization.k8s.io/leader-election-rolebinding created
rolebinding.rbac.authorization.k8s.io/rolebinding-priv-scc-podconfig-operator created
rolebinding.rbac.authorization.k8s.io/manager-rolebinding created
clusterrolebinding.rbac.authorization.k8s.io/manager-rolebinding created
deployment.apps/podconfig-operator created
```
Then check the operator on your cluster.
Let's move to our test namespace `cnf-test`
```
oc project cnf-test

Now using project "cnf-test" on server "https://<your cluster api url should appear here>
```
Run `oc get pods` and you should be able to see something like below:
```
podconfig-operator-78c88b566d-pm5zr   1/1     Running   0          4m59s 
``` 
---
### Usage

Once your operator is running we have a couple of sample configurations that can be applied to unprivileged pods. For the sake of testing and proving the concept the operator spawns 2 unprivileged pod replicas and performs the configurations on top of those replicas. Let's see how it happens.

First let's check our sample podConfig and understand what it's going to do. If you are familiar with the multi-CNI plugin called Multus you will recognize that language on the yaml manifest below. Let's check the fields present there for the moment.
```
apiVersion: podconfig.opdev.io/v1alpha1
kind: PodConfig
metadata:
  name: podconfig-sample-a
spec:

  sampleDeployment:
    create: true
    name: cnf-example-a
    
  networkAttachments:
    - name: pc0
      linkType: veth
      master: pcbr0
      parent: pc0
      cidr: "192.168.100.0/24"
    - name: pc1
      linkType: veth
      master: pcbr1
      parent: pc1
      cidr: "192.168.99.0/24"    
```
***sampleDeployment***
> The sampleDeployment is just a shortcut to deploy pods for demonstration. It deploys completely unprivilged pods with all the necessary tools to at least see what has been changed in the pod's configuration.

  `create:` a boolean that triggers the deployment
  `name:` simply the deployment name

***networkAttachments***
> On network attachments we have access to all network stack already offered by a Linux system. For the moment the attachment working is the most simple veth pair. It creates on the fly at runtime a new network inside a pod with the given parameters and deletes it when it's not needed anymore.

`name:` that is the prefix appended to the process id of the Pod Veth pair's end. With that we guarantee the uniqueness of that new interface.
`linkType:` it could any type supplied by the iproute2 family of commands in Linux or any extra custom types created almost as plugin to this interface.
`master:` here we're talking about the switching device that will receive and forward the packet at node/host level. At this point in time it's a simple Linux bridge but any other data plane can be added to this scheme.
`parent:` If creating subinterfaces or virtual interfaces that rely on a parent interface to encapsulate packets such as a VLAN or VFVLAN interface, here is where the parent interface goes. With veth pairs the created interfaces are the parent's themselves.
`cidr:` The network address range to be used for that new network. At this point in time it's a mockup library that handles only /24 networks as if it is an IPAM software. <b>Even for testing I recommend checking the network cluster operator in OpenShift to make sure there is no 192.168.*.0/24 network in your cluster.</b> To do that just try `oc describe network cluster` and you should be able to see both the cluster network and pods networks in use. More discussion on that subject can be found on [Design Proposal](design_proposal.md).

In summary what this `podconfig-sample-a` is going to do is deploy 2 unprivileged pods and configure 2 extra networks for each one.

Let's run it:
```
oc apply -f config/samples/podconfig_v1alpha1_podconfig-a.yaml
```
After that we should see something like this:

`oc get pods`

```
NAME                                  READY   STATUS    RESTARTS   AGE
cnf-example-a-846566d4fb-7rxft        1/1     Running   0          50s
cnf-example-a-846566d4fb-lmxmg        1/1     Running   0          50s
podconfig-operator-78c88b566d-pm5zr   1/1     Running   0          31m
```
Let's take a look inside one of the pods:

```
oc exec -it cnf-example-a-846566d4fb-7rxft -- /bin/bash
bash-5.0$
```

Now let's check the interfaces:
```
bash-5.0$ ip addr
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host
       valid_lft forever preferred_lft forever
3: eth0@if5787: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 8951 qdisc noqueue state UP group default
    link/ether 0a:58:0a:80:02:b4 brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 10.128.2.180/23 brd 10.128.3.255 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fe80::4cbf:42ff:fef5:13f0/64 scope link
       valid_lft forever preferred_lft forever
5: pc02841804@if5790: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether c6:f4:a6:0e:e3:51 brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 192.168.100.2/24 brd 192.168.100.255 scope global pc02841804
       valid_lft forever preferred_lft forever
    inet6 fe80::c4f4:a6ff:fe0e:e351/64 scope link
       valid_lft forever preferred_lft forever
7: pc12841804@if5792: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether 4a:87:f4:8b:31:fa brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 192.168.99.2/24 brd 192.168.99.255 scope global pc12841804
       valid_lft forever preferred_lft forever
    inet6 fe80::4887:f4ff:fe8b:31fa/64 scope link
       valid_lft forever preferred_lft forever
```

We can see 2 other interfaces running on the pods beyond the regular eth0. And both new interfaces are configured the way we asked for. Now let's check the node where those pods are running:

```
oc debug node/<your node url here>
```
```
Starting pod/<your debug url will show up here> ...
To use host binaries, run `chroot /host`
Pod IP: 10.0.229.189
If you don't see a command prompt, try pressing enter.
sh-4.2#
```
Let's go root on host and take a shell on it:
```
sh-4.2# chroot /host /bin/bash
[root@ip-10-0-229-189 /]#
```
Now we check the network for new bridges with the master name we put in:
```
ip addr | grep -A 4 pcbr
```
We can see now the 2 created bridges and also all 4 veth pair host end configured on the OpenShift node.
```
5789: pcbr0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether 72:da:49:9a:e3:ba brd ff:ff:ff:ff:ff:ff
    inet 192.168.100.1/24 brd 192.168.100.255 scope global pcbr0
       valid_lft forever preferred_lft forever
    inet6 fe80::cc05:46ff:fe57:2232/64 scope link
       valid_lft forever preferred_lft forever
5790: hpc02841804@if5: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue master pcbr0 state UP group default
    link/ether c2:9f:e9:16:a7:e0 brd ff:ff:ff:ff:ff:ff link-netns 8817dfad-3f15-4946-9d0f-86564cb3c374
    inet6 fe80::c09f:e9ff:fe16:a7e0/64 scope link
       valid_lft forever preferred_lft forever
5791: pcbr1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether 02:ee:e4:30:99:1d brd ff:ff:ff:ff:ff:ff
    inet 192.168.99.1/24 brd 192.168.99.255 scope global pcbr1
       valid_lft forever preferred_lft forever
    inet6 fe80::e092:f8ff:fea3:15b4/64 scope link
       valid_lft forever preferred_lft forever
5792: hpc12841804@if7: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue master pcbr1 state UP group default
    link/ether 6a:35:17:9a:ce:04 brd ff:ff:ff:ff:ff:ff link-netns 8817dfad-3f15-4946-9d0f-86564cb3c374
    inet6 fe80::6835:17ff:fe9a:ce04/64 scope link
       valid_lft forever preferred_lft forever
5793: hpc02841798@if5: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue master pcbr0 state UP group default
    link/ether 72:da:49:9a:e3:ba brd ff:ff:ff:ff:ff:ff link-netns 4083afad-eb37-4560-9a08-b217891a895e
    inet6 fe80::70da:49ff:fe9a:e3ba/64 scope link
       valid_lft forever preferred_lft forever
5794: hpc12841798@if7: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue master pcbr1 state UP group default
    link/ether 02:ee:e4:30:99:1d brd ff:ff:ff:ff:ff:ff link-netns 4083afad-eb37-4560-9a08-b217891a895e
    inet6 fe80::ee:e4ff:fe30:991d/64 scope link
       valid_lft forever preferred_lft forever
5795: vethc1ca7566@if3: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 8951 qdisc noqueue state UP group default
```
Since our bridges have ips 192.168.100.1 and 192.168.99.1 let's ping them from our pod. Get back to your pod oc exec terminal and do:

```
bash-5.0$ ping 192.168.100.1
PING 192.168.100.1 (192.168.100.1) 56(84) bytes of data.
64 bytes from 192.168.100.1: icmp_seq=1 ttl=64 time=0.087 ms
64 bytes from 192.168.100.1: icmp_seq=2 ttl=64 time=0.027 ms
64 bytes from 192.168.100.1: icmp_seq=3 ttl=64 time=0.037 ms
64 bytes from 192.168.100.1: icmp_seq=4 ttl=64 time=0.035 ms
64 bytes from 192.168.100.1: icmp_seq=5 ttl=64 time=0.033 ms
^C
--- 192.168.100.1 ping statistics ---
5 packets transmitted, 5 received, 0% packet loss, time 4099ms
rtt min/avg/max/mdev = 0.027/0.043/0.087/0.021 ms
```
And ...
```
bash-5.0$ ping 192.168.99.1
PING 192.168.99.1 (192.168.99.1) 56(84) bytes of data.
64 bytes from 192.168.99.1: icmp_seq=1 ttl=64 time=0.083 ms
64 bytes from 192.168.99.1: icmp_seq=2 ttl=64 time=0.046 ms
64 bytes from 192.168.99.1: icmp_seq=3 ttl=64 time=0.032 ms
64 bytes from 192.168.99.1: icmp_seq=4 ttl=64 time=0.063 ms
64 bytes from 192.168.99.1: icmp_seq=5 ttl=64 time=0.051 ms
^C
--- 192.168.99.1 ping statistics ---
5 packets transmitted, 5 received, 0% packet loss, time 4118ms
rtt min/avg/max/mdev = 0.032/0.055/0.083/0.017 ms
```
What about the other pod? What IP addresses do we have there?
```
oc exec -it cnf-example-a-846566d4fb-lmxmg -- /bin/bash
bash-5.0$ ip addr
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host
       valid_lft forever preferred_lft forever
3: eth0@if5788: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 8951 qdisc noqueue state UP group default
    link/ether 0a:58:0a:80:02:b5 brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 10.128.2.181/23 brd 10.128.3.255 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fe80::7096:70ff:fe96:aa4d/64 scope link
       valid_lft forever preferred_lft forever
5: pc02841798@if5793: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether 72:4f:f9:80:f5:68 brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 192.168.100.3/24 brd 192.168.100.255 scope global pc02841798
       valid_lft forever preferred_lft forever
    inet6 fe80::704f:f9ff:fe80:f568/64 scope link
       valid_lft forever preferred_lft forever
7: pc12841798@if5794: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether 0a:d4:90:5b:fb:f0 brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 192.168.99.3/24 brd 192.168.99.255 scope global pc12841798
       valid_lft forever preferred_lft forever
    inet6 fe80::8d4:90ff:fe5b:fbf0/64 scope link
       valid_lft forever preferred_lft forever
```
Let's try to ping from this one to the other:
```
PING 192.168.100.2 (192.168.100.2) 56(84) bytes of data.
64 bytes from 192.168.100.2: icmp_seq=1 ttl=64 time=0.060 ms
64 bytes from 192.168.100.2: icmp_seq=2 ttl=64 time=0.035 ms
64 bytes from 192.168.100.2: icmp_seq=3 ttl=64 time=0.034 ms
64 bytes from 192.168.100.2: icmp_seq=4 ttl=64 time=0.034 ms
64 bytes from 192.168.100.2: icmp_seq=5 ttl=64 time=0.040 ms
^C
--- 192.168.100.2 ping statistics ---
5 packets transmitted, 5 received, 0% packet loss, time 4133ms
rtt min/avg/max/mdev = 0.034/0.040/0.060/0.010 ms
```
And ...
```
bash-5.0$ ping 192.168.99.2
PING 192.168.99.2 (192.168.99.2) 56(84) bytes of data.
64 bytes from 192.168.99.2: icmp_seq=1 ttl=64 time=0.063 ms
64 bytes from 192.168.99.2: icmp_seq=2 ttl=64 time=0.037 ms
64 bytes from 192.168.99.2: icmp_seq=3 ttl=64 time=0.039 ms
64 bytes from 192.168.99.2: icmp_seq=4 ttl=64 time=0.036 ms
64 bytes from 192.168.99.2: icmp_seq=5 ttl=64 time=0.042 ms
^C
--- 192.168.99.2 ping statistics ---
5 packets transmitted, 5 received, 0% packet loss, time 4077ms
rtt min/avg/max/mdev = 0.036/0.043/0.063/0.010 ms
```
There we go! Both networks are functional and they were configured after pod creation. That is easier to see if you deploy your own unprivileged pods. The automatic deployment is here only to speed up testing. When deleting the CR all configuration will be delete before from both the pods and the node without disruption the normal work of the cluster.

But now let's check what Kubernetes API gives to us when we deploy the podConfig:

Run `oc get podconfig`
```
NAME                 AGE
podconfig-sample-a   18m
```
Other podconfigs with different configuration lists and tasks may be applied to the cluster and they will also appear listed in the section above.

Run `oc describe podconfig`
```
oc describe podconfig
Name:         podconfig-sample-a
Namespace:    cnf-test
Labels:       <none>
Annotations:  kubectl.kubernetes.io/last-applied-configuration:
                {"apiVersion":"podconfig.opdev.io/v1alpha1","kind":"PodConfig","metadata":{"annotations":{},"name":"podconfig-sample-a","namespace":"cnf-t...
API Version:  podconfig.opdev.io/v1alpha1
Kind:         PodConfig
Metadata:
  Creation Timestamp:  2020-11-17T03:54:43Z
  Finalizers:
    podconfig.finalizers.opdev.io
  Generation:  1

  < ... omitting a few bits here for brevity ...>

Spec:
  Network Attachments:
    Cidr:       192.168.100.0/24
    Link Type:  veth
    Master:     pcbr0
    Name:       pc0
    Parent:     pc0
    Cidr:       192.168.99.0/24
    Link Type:  veth
    Master:     pcbr1
    Name:       pc1
    Parent:     pc1
  Sample Deployment:
    Create:  true
    Name:    cnf-example-a
Status:
  Phase:  configured
  Pod Configurations:
    Config List:
      {podVethName:pc02841804 podIPAddr:192.168.100.2/24 peerVethName:hpc02841804 bridge:pcbr0}
      {podVethName:pc12841804 podIPAddr:192.168.99.2/24 peerVethName:hpc12841804 bridge:pcbr1}
    Pod Name:  cnf-example-a-846566d4fb-7rxft
    Config List:
      {podVethName:pc02841798 podIPAddr:192.168.100.3/24 peerVethName:hpc02841798 bridge:pcbr0}
      {podVethName:pc12841798 podIPAddr:192.168.99.3/24 peerVethName:hpc12841798 bridge:pcbr1}
    Pod Name:  cnf-example-a-846566d4fb-lmxmg
```
Check that you can see the configurations applied per Pod with the pod names in the status field. And that's for now. Many other important pieces of information may be put in there to help unprivileged app admins managed the custom configs for their pods.

#### Other Links

[Design Proposal](docs/design_proposal.md)

[License](LICENSE)
