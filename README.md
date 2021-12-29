# kube-parrot

A Kubernetes Controller that Dynamically Announces Routes with BGP

## Local testing

```
╭― ― ― ― ― ― ― ― ―╮  ╭― ― ― ― ― ― ― ― ―╮
│  arista-ceos1   │  │  arista-ceos2   │
│    10.0.0.2     │  │    10.0.0.3     │
╰― ― ― ― ― ― ― ― ―╯  ╰― ― ― ― ― ― ― ― ―╯
            |            |
            |     iBGP   |     ↑
         ╭― ― ― ― ― ― ― ― ―╮   podcidr    169.0.0.0/24
         │       k3s       │   externalIP 10.0.0.100 
         │    10.0.0.10    │   any other svc w/ externalIP
         ╰― ― ― ― ― ― ― ― ―╯
```

Running `make lab` will build an image (mind the registry) and deploy kube-parrot to k3s container connected to two Arista cEOS switches. 

```
› make pods
kubectl --kubeconfig kubeconfig.yaml --context default get pod -n kube-system
NAME                                      READY   STATUS    RESTARTS   AGE
kube-parrot-2722b                         1/1     Running   0          56s
metrics-server-86cbb8457f-q5qzb           1/1     Running   0          56s
coredns-6488c6fcc6-577rs                  1/1     Running   0          56s
nginx-deployment-66b6c48dd5-qw6hs         1/1     Running   0          56s
local-path-provisioner-5ff76fc89d-wdzwg   1/1     Running   0          56s
helm-install-traefik-tr2s8                1/1     Running   0          56s


› make logs kube-parrot-2722b
kubectl --kubeconfig kubeconfig.yaml --context default -n kube-system logs kube-parrot-2722b
Welcome to Kubernetes Parrot v202112291324
I1229 12:51:33.506280       1 server.go:80] Adding Neighbor: 10.0.0.2
time="2021-12-29T12:51:33Z" level=info msg="Add a peer configuration for:10.0.0.2" Topic=Peer
I1229 12:51:33.556676       1 server.go:80] Adding Neighbor: 10.0.0.3
time="2021-12-29T12:51:33Z" level=info msg="Add a peer configuration for:10.0.0.3" Topic=Peer
I1229 12:51:33.667271       1 metrics.go:25] Serving Prometheus metrics on 10.0.0.10:30039
I1229 12:51:33.668747       1 store.go:44] Announcing         169.0.0.0/24 -> 10.0.0.10       (NodePodSubnet: 169.0.0.0/24 -> 5a46ee61b9e6)
time="2021-12-29T12:51:49Z" level=info msg="Peer Up" Key=10.0.0.3 State=BGP_FSM_OPENCONFIRM Topic=Peer
time="2021-12-29T12:51:52Z" level=info msg="Peer Up" Key=10.0.0.2 State=BGP_FSM_OPENCONFIRM Topic=Peer
I1229 12:51:52.674222       1 store.go:44] Announcing        10.0.0.100/32 -> 10.0.0.10       (ExternalIP:    kube-system/nginx -> 10.0.0.10)


# check from switch side
› make sw1
docker exec -it arista-sw1 Cli
SW1>en
SW1#show ip bgp
BGP routing table information for VRF default
Router identifier 10.0.0.2, local AS number 65001
Route status codes: s - suppressed, * - valid, > - active, # - not installed, E - ECMP head, e - ECMP
                    S - Stale, c - Contributing to ECMP, b - backup, L - labeled-unicast
Origin codes: i - IGP, e - EGP, ? - incomplete
AS Path Attributes: Or-ID - Originator ID, C-LST - Cluster List, LL Nexthop - Link Local Nexthop

         Network                Next Hop            Metric  LocPref Weight  Path
 * >     10.0.0.100/32          10.0.0.10             0       100     0       i
 * >     169.0.0.0/24           10.0.0.10             0       100     0       i
```
