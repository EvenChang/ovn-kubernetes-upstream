# Allocate Virtual Functions to Pods in a single Kubernetes cluster with OVN-Kubernetes CNI
The purpose of this document is to demonstrate how to use DPU to allocate a Virtual Function as a Pod's primary interface to speed up network using the DPU hardware offload feature.

## Prerequisites
-	Open vSwitch 2.13 or above
-	iproute >= 4.12
-	sriov-device-plugin
-	multus-cni
-	BlueField-2 ConnectX-6 Dx (DPU)
-	ovn-kubernetes image: ghcr.io/ovn-org/ovn-kubernetes/ovn-kube-ubuntu:release-1.0

## Host Environment
#### Control Plane Node
- OS Version: Ubuntu 20.04.6 LTS
- Linux Kernel Version: 5.4.0-186-generic
- IP Address: 192.168.40.111/22

#### Worker-host Node
- OS Version: Ubuntu 22.04.2 LTS
- Linux Kernel Version: 5.15.0-112-generic
- IP Address: 192.168.42.201/22

#### Worker-dpu Node
- OS Version: Ubuntu 22.04.3 LTS
- Linux Kernel Version: 5.15.0-1021-bluefield
- IP Address: 192.168.41.127/22

## Architecture
In this environment, there are two computers: one is an ordinary computer and the other has a BlueField-2 CX6 Card.

### K8S Cluster

Deploy K8S Cluster with three nodes.
-	Control Plane Node: Ordinary computer
-	Worker-host Node:  Host
-	Worker-dpu Node:   DPU

![accton dpu architecture](/docs/blog/accton-dpu-architecture.png)



### Master Node
It should have basic K8S components like the K8S API server.
OVN-Kubernetes component should be deployed:
1.	**ovnkube-node (full mode)**: OVN-Kubernetes node agent.
Note: Full mode means you don't need to set the OVNKUBE_NODE_MODE argument. 
2.	**ovnkube-master**: ovn-kubernetes master controller, it is the integration point with OVN and is used to program OVN in order to satisfy kubernetes network requirements.
3.	**ovnkube-db**: An ovsdb which store OVN (Open Virtual Network) Northbound (NB) and Southbound (SB) databases.
4.	**ovs-node**: The Open vSwitch (OVS) components running on a node.

### Worker-host Node
1.	ovnkube-node (dpu-host mode): OVN-Kubernetes node agent.
Note: dpu-host mode means you need to set the OVNKUBE_NODE_MODE argument as "dpu-host". 
The Worker-host should not have Open vSwitch(OVS) installed. Packet processing is handled by the DPU's eSwitch.


### Worker-dpu Node
1.	**ovnkube-node (dpu mode)**: OVN-Kubernetes node agent.
Note: dpu mode means you need to set the OVNKUBE_NODE_MODE argument as "dpu". 
2.	**ovn-controller**: OVN node agent, translates logical flows from OVN South db to logical flows in OVS. ovn-controller is moved into DPU.


### Worker-host SR-IOV Configuration
- Check the Number of VF Supported on the NIC
```
cat /sys/class/net/enp2s0f0np0/device/sriov_totalvfs
16
```

-	Create the VFs
```
echo '3' > /sys/class/net/enp2s0f0np0/device/sriov_numvfs
```

-	Verify the VFs are created
```
sudo ip link show enp2s0f0np0
3: enp2s0f0np0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP mode DEFAULT group default qlen 1000
    link/ether 10:70:fd:09:81:aa brd ff:ff:ff:ff:ff:ff
    vf 0  link/ether 00:00:00:00:00:00 brd ff:ff:ff:ff:ff:ff, spoof checking off, link-state auto, trust off, query_rss off
    vf 1  link/ether 00:00:00:00:00:00 brd ff:ff:ff:ff:ff:ff, spoof checking off, link-state auto, trust off, query_rss off
    vf 2  link/ether 00:00:00:00:00:00 brd ff:ff:ff:ff:ff:ff, spoof checking off, link-state auto, trust off, query_rss off
```

-	Verify the VFs PCI devices information
```
lspci -nnD
0000:02:00.3 Ethernet controller [0200]: Mellanox Technologies ConnectX Family mlx5Gen Virtual Function [15b3:101e] (rev 01)
0000:02:00.4 Ethernet controller [0200]: Mellanox Technologies ConnectX Family mlx5Gen Virtual Function [15b3:101e] (rev 01)
0000:02:00.5 Ethernet controller [0200]: Mellanox Technologies ConnectX Family mlx5Gen Virtual Function [15b3:101e] (rev 01)
```

-	Setup the PF to be up
```
sudo ip link set enp2s0f0np0 up
```

-	Enable hardware offload
```
sudo ethtool -K enp2s0f0np0 hw-tc-offload on
```

## Worker-dpu Open vSwitch enable hardware offload and relate configurations

-	Set DPU-related information 
```
sudo ovs-vsctl set Open_vSwitch . external-ids:host-k8s-nodename="dpu-host"
sudo ovs-vsctl set Open_vSwitch . external-ids:ovn-gw-interface="p0" 
sudo ovs-vsctl set Open_vSwitch . external-ids:ovn-gw-nexthop="192.168.40.254"
```

-	Set hw-offload=true and restart Open vSwitch
```
systemctl enable openvswitch-switch.service
ovs-vsctl set Open_vSwitch . other_config:hw-offload=true
systemctl restart openvswitch-switch.service
```

-   Create a bridge and attach both the uplink and the PF representor on the DPU
```
ubuntu@dpu:~$  sudo ovs-vsctl add-br brp0
ubuntu@dpu:~$  sudo ip addr add 192.168.41.127/22 dev brp0
ubuntu@dpu:~$  sudo ovs-vsctl add-port brp0 p0
ubuntu@dpu:~$  sudo ovs-vsctl add-port brp0 pf0hpf
ubuntu@dpu:~$  sudo ovs-vsctl show
aa35917b-16f3-43a8-869a-e169d6a117e3
    Bridge brp0
        Port p0
            Interface p0
        Port pf0hpf
            Interface pf0hpf
        Port brp0
            Interface brp0
                type: internal
```

## Deploy Kubernetes Cluster
Use kubeadm to deploy K8S Cluster
-	Kubernetes Version: v1.25

### Master and Worker-host
```
sudo apt-get update
sudo apt-get install -y apt-transport-https ca-certificates curl

curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.25/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg

echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.25/deb/ /' | sudo tee /etc/apt/sources.list.d/kubernetes.list

sudo apt-get update
sudo apt-get install -y kubelet kubeadm kubectl
```

### Worker-dpu
```
sudo apt-get update 
sudo apt-get install -y apt-transport-https ca-certificates curl 
curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.25/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg 
echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.25/deb/ /' | sudo tee /etc/apt/sources.list.d/kubernetes.list 
sudo apt-get update 
sudo apt-get install -y kubeadm kubectl
Note: DPU ARM OS kubelet includes by default.
```

### Create K8S Cluster by kubeadm
-	Master Host
```
sudo kubeadm init --skip-phases=addon/kube-proxy
```

-	Worker-host node
```
sudo kubeadm join 192.168.40.111:6443 --token anedng.pgt7hju8xc29kam7 --discovery-token-ca-cert-hash sha256:fe22ced72c233ecbd70b13514fa51820b1ffd7798d4b346ad6af640e31115946
```

-	Worker-dpu node
```
sudo swapoff -a
sudo modprobe br_netfilter
sudo sysctl -w net.ipv4.ip_forward=1
sudo kubeadm join 192.168.40.111:6443 --token anedng.pgt7hju8xc29kam7 --discovery-token-ca-cert-hash sha256:fe22ced72c233ecbd70b13514fa51820b1ffd7798d4b346ad6af640e31115946
```

## Deploy OVN-Kubernetes CNI
### Download Repository and Generate YAML file
-	Clone OVN-Kubernetes Repository
```
mkdir -p $HOME/work/src/github.com/ovn-org 
cd $HOME/work/src/github.com/ovn-org 
git clone https://github.com/ovn-org/ovn-kubernetes 
cd $HOME/work/src/github.com/ovn-org/ovn-kubernetes/dist/images 
```

-	Generate OVN-Kubernetes deployment YAML file
```
./daemonset.sh --image=ghcr.io/ovn-org/ovn-kubernetes/ovn-kube-ubuntu:release-1.0 --net-cidr=10.233.64.0/18 --svc-cidr=10.96.0.0/12 --gateway-mode="shared" --k8s-apiserver=https://192.168.40.111:6443 --enable-ovnkube-identity=false
```

### Build OVN-Kubernetes image (Optional)
If you don't need to build the OVN-Kubernetes image, you can skip this step. However, if you are trying to use a different commit, then you should build the image yourself.
```
cd $HOME/work/src/github.com/ovn-org/ovn-kubernetes/go-controller
make
cp _output/go/bin/* ../dist/images/
cd $HOME/work/src/github.com/ovn-org/ovn-kubernetes/go-controller/dist/image
make ubuntu
Note: Remember that the DPU uses the `ovn-kube-u-arm64` image instead of `ovn-kube-u`.
```

### Pull OVN-Kubernetes image on DPU Node
```
DPU node should pull arm base image
docker pull ghcr.io/ovn-org/ovn-kubernetes/ovn-kube-ubuntu:release-1.0@sha256:18b11416fc678af0cc7b18d2fa6dd75ded0631baab756314cd147520ebed7ac3 
docker tag <imageId> ghcr.io/ovn-org/ovn-kubernetes/ovn-kube-ubuntu:release-1.0
docker image save -o ovn-k.tar ghcr.io/ovn-org/ovn-kubernetes/ovn-kube-ubuntu:release-1.0
sudo ctr -n=k8s.io images import ovn-k.tar
```

### OVN-Kubernetes YAML file Configuration
Due to the DPU environment, it is necessary to customize arguments `ovnkube-node-dpu-host.yaml' files

#### ovnkube-master.yaml
```
- name: OVN_DISABLE_REQUESTEDCHASSIS
  value: "true"
```

#### ovnkube-node.yaml
-	Only schedule the pod on control-plane node.
```
nodeSelector: 
  node-role.kubernetes.io/control-plane: ""
```

#### ovs-node.yaml
-	In our scenario, it is only scheduled on the control-plane node because our control-plane node does not run OVS directly.
    If a node is running OVS directly, do not apply this YAML file.
```
nodeSelector: 
  node-role.kubernetes.io/control-plane: ""
```


#### ovnkube-node-dpu-host.yaml
- Only a ovnkube-node container
```
- name: OVNKUBE_NODE_MGMT_PORT_NETDEV
  value: "enp2s0f0v0"  # Specify a Virtual Function as management port.
```

### Deploy OVN-Kubernetes YAML file
-	Deploy OVN-Kubernetes related files
```
cd $HOME/work/src/github.com/ovn-org/ovn-kubernetes/dist/yaml
kubectl create -f ovn-setup.yaml
for yaml in `echo rbac-ovnkube-cluster-manager.yaml rbac-ovnkube-db.yaml rbac-ovnkube-identity.yaml rbac-ovnkube-master.yaml rbac-ovnkube-node.yaml k8s.ovn.org_adminpolicybasedexternalroutes.yaml`; do kubectl create -f $yaml
done

kubectl create -f ovnkube-db.yaml
kubectl create -f ovnkube-master.yaml
kubectl create -f ovnkube-node.yaml
kubectl create -f ovs-node.yaml
kubectl create -f ovnkube-node-dpu-host.yaml
kubectl create -f ovnkube-node-dpu.yaml
```

-	Label the dpu-host node and the dpu node for scheduling related pods.
```
kubectl label nodes dpu-host k8s.ovn.org/dpu-host= 
kubectl label nodes dpu k8s.ovn.org/dpu=
```

-	Check if the ovn-kubernetes Pods are running normally
```
ovn-kubernetes   ovnkube-db-5cdd697f5f-pf6rx       2/2  Running           
ovn-kubernetes   ovnkube-master-758677bc67-ksdgl  2/2  Running  
ovn-kubernetes   ovnkube-node-6xxzj                3/3  Running   
ovn-kubernetes   ovnkube-node-dpu-2qvs9           3/3  Running    
ovn-kubernetes   ovnkube-node-dpu-host-kvn66      1/1   Running     
ovn-kubernetes   ovs-node-ct9b2                   1/1   Running
```

## Deploy Multus-cni
-	Deploy Multus-cni
```
kubectl apply -f https://raw.githubusercontent.com/k8snetworkplumbingwg/multus-cni/master/deployments/multus-daemonset-thick.yml
```

-	Add NetworkAttachmentDefinition Resource.
```
apiVersion: "k8s.cni.cncf.io/v1"
kind: NetworkAttachmentDefinition
metadata:
  name: default
  namespace: kube-system
  annotations:
    k8s.v1.cni.cncf.io/resourceName: mellanox.com/mlnx_sriov_cx6
spec:
  config: '{"cniVersion":"0.3.1","name":"ovn-kubernetes","type":"ovn-k8s-cni-overlay","ipam":{},"dns":{}}'
```

## Deploy SR-IOV Device Plugin
-	Git clone SR-IOV Device Plugin repository
```
git clone https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin.git
```

Before configuring the SR-IOV Device configMap, please check the information about your NIC.
```
lspci -nnvD
0000:02:00.4 Ethernet controller [0200]: Mellanox Technologies ConnectX Family mlx5Gen Virtual Function [15b3:101e] (rev 01)
    Subsystem: Mellanox Technologies ConnectX Family mlx5Gen Virtual Function [15b3:0062]
    Flags: bus master, fast devsel, latency 0
    Memory at 96200000 (64-bit, prefetchable) [virtual] [size=2M]
    Capabilities: [60] Express Endpoint, MSI 00
    Capabilities: [9c] MSI-X: Enable+ Count=12 Masked-
    Capabilities: [100] Vendor Specific Information: ID=0000 Rev=0 Len=00c <?>
    Capabilities: [150] Alternative Routing-ID Interpretation (ARI)
    Kernel driver in use: mlx5_core
    Kernel modules: mlx5_core
```

In the previous section, three Virtual Functions were created.
One of the Virtual Function will be configured as the OVN Kubernetes management port. Therefore, we configure two VFs in this configMap.
```
cd sriov-network-device-plugin/deployments
```
-	Modify the configMap.yaml file accordingly.
```
apiVersion: v1
kind: ConfigMap
metadata:
  name: sriovdp-config
  namespace: kube-system
data:
  config.json: |
    {
        "resourceList": [{
                "resourceName": "mlnx_sriov_cx6",
                "selectors": {
                    "vendors": ["15b3"],
                    "devices": ["101e"],
                    "drivers": ["mlx5_core"],
                    "pfNames": ["enp2s0f0np0#1-2"]
                }
            }
        ]
    }
```
  Note: The `pfNames` specify which VFs can be allocated as resources for the Pod, omitting `VF0`, which is used for `ovn-k8s-mp0`.

-	Apply configMap and SR-IOV daemonset.
```
kubectl apply -f configMap.yaml
kubectl apply -f sriovdp-daemonset.yaml
```

-	Verify that the VF is available as a resource on the host-dpu worker node.
```
kubectl describe nodes host-dpu
Capacity:
  mellanox.com/mlnx_sriov_cx6:  2
Allocatable:
  mellanox.com/mlnx_sriov_cx6:  2
```

## Deploy POD with OVS hardware-offload

-	Create POD spec
```
apiVersion: v1
kind: Pod
metadata:
  name: ovs-offload-pod1
  namespace: ovn-kubernetes
  annotations:
    v1.multus-cni.io/default-network: default
spec:
  containers:
  - name: appcntr1
    image: alpine
    command:
      - "sleep"
      - "604800"
    imagePullPolicy: IfNotPresent
    resources:
      requests:
        mellanox.com/mlnx_sriov_cx6: '1'
      limits:
        mellanox.com/mlnx_sriov_cx6: '1'
```

-	You can verify the offloaded packets by using this command on the DPU
```
sudo ovs-appctl dpctl/dump-flows type=offloaded
```

- The OVS output on the DPU should look like this
```
ubuntu@dpu:~$ sudo ovs-vsctl show
aa35917b-16f3-43a8-869a-e169d6a117e3
    Bridge brp0
        Port pf0hpf
            Interface pf0hpf
        Port brp0
            Interface brp0
                type: internal
        Port patch-brp0_host-dpu-to-br-int
            Interface patch-brp0_host-dpu-to-br-int
                type: patch
                options: {peer=patch-br-int-to-brp0_host-dpu}
        Port p0
            Interface p0
    Bridge br-int
        fail_mode: secure
        datapath_type: system
        Port pf0vf2
            Interface pf0vf2
        Port patch-br-int-to-brp0_host-dpu
            Interface patch-br-int-to-brp0_host-dpu
                type: patch
                options: {peer=patch-brp0_host-dpu-to-br-int}
        Port pf0vf1
            Interface pf0vf1
        Port ovn-8c1016-0
            Interface ovn-8c1016-0
                type: geneve
                options: {csum="true", key=flow, local_ip="192.168.41.127", remote_ip="192.168.40.111"}
        Port ovn-k8s-mp0
            Interface ovn-k8s-mp0
        Port br-int
            Interface br-int
                type: internal
```
