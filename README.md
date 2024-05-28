# kubectl-gather

This is a kubectl plugin for gathering data about your cluster that may
help to debug issues.

Kubernetes is big and complicated, and when something breaks it is hard
to tell which info is needed for debugging. Even if you known which
resources or logs are needed, it is hard to get the data manually. When
working with multiple related clusters gathering the right data from the
right cluster is even harder.

The `kubectl gather` tool makes it easy to gather data quickly from
multiple clusters with a single command. It gathers *all* resources from
*all* clusters. It also gather related data such as pods logs, on for
specific cases, external logs stored on the nodes. The data is stored in
a local directory, one file per resource, making it easy to navigate and
inspect using standard tools. If you know what you want to gather, it
is much faster and consume fraction of the storage to gather only
specific namespaces from all clusters.

## Installing

You need to install using the go command. If you don't have Go
installed, install it first. On rpm based system:

```
sudo dnf install golang
```

Then install latest version of the plugin:

```
go install github.com/nirs/kubectl-gather@latest
```

The plugin is installed in `$HOME/go/bin`. Make sure this directory is
in the PATH. If not, add this line to your `~/.bashrc`:

```
PATH="$HOME/go/bin:$PATH"
```

## Gathering everything from the current cluster

The simplest way is to gather everything from the current cluster named
"hub":

```
$ kubectl gather -d gather.one
2024-05-27T23:03:58.838+0300	INFO	gather	Using kubeconfig "/home/nsoffer/.kube/config"
2024-05-27T23:03:58.840+0300	INFO	gather	Using current context "hub"
2024-05-27T23:03:58.841+0300	INFO	gather	Gathering from all namespaces
2024-05-27T23:03:58.841+0300	INFO	gather	Gathering from cluster "hub"
2024-05-27T23:04:00.219+0300	INFO	gather	Gathered 1439 resources from cluster "hub" in 1.379 seconds
2024-05-27T23:04:00.220+0300	INFO	gather	Gathered 1439 resources from 1 clusters in 1.379 seconds
```

This gathers 15 MiB of data into the directory "gather.one":

```
$ du -sh gather.one/
15M	gather.one/
```

The "cluster" directory contains the cluster scope resources, and the
"namespaces" directory contains the namespaced resources:

```
$ tree -L 2 gather.one/
gather.one/
├── gather.log
└── hub
    ├── cluster
    └── namespaces
```

Here is example content from the "pods" directory in the "ramen-system"
namespace:

```
$ tree gather.one/hub/namespaces/ramen-system/pods/
gather.one/hub/namespaces/ramen-system/pods/
├── ramen-hub-operator-84d7dc89bd-7qkwm
│   ├── kube-rbac-proxy
│   │   └── current.log
│   └── manager
│       └── current.log
└── ramen-hub-operator-84d7dc89bd-7qkwm.yaml
```

We can use standard tools to inspect the data. In this example we grep
all current and previous logs in all namespaces:

```
$ grep WARN gather.one/hub/namespaces/*/pods/*/*/*.log
gather.one/hub/namespaces/kube-system/pods/coredns-7db6d8ff4d-9cj6c/coredns/current.log:[WARNING] plugin/kubernetes: starting server with unsynced Kubernetes API
gather.one/hub/namespaces/kube-system/pods/kube-controller-manager-hub/kube-controller-manager/current.log:E0527 19:52:08.593071       1 core.go:105] "Failed to start service controller" err="WARNING: no cloud provider provided, services of type LoadBalancer will fail" logger="service-lb-controller"
```

## Gathering data from multiple clusters

In this example we have 3 clusters configured for disaster recovery:

```
$ kubectl config get-contexts
CURRENT   NAME          CLUSTER         AUTHINFO        NAMESPACE
          dr1           dr1             dr1             default
          dr2           dr2             dr2             default
*         hub           hub             hub             default
```

To gather data from all clusters run:

```
$ kubectl gather --contexts hub,dr1,dr2 -d gather.all
2024-05-27T23:16:16.459+0300	INFO	gather	Using kubeconfig "/home/nsoffer/.kube/config"
2024-05-27T23:16:16.460+0300	INFO	gather	Gathering from all namespaces
2024-05-27T23:16:16.460+0300	INFO	gather	Gathering from cluster "hub"
2024-05-27T23:16:16.461+0300	INFO	gather	Gathering from cluster "dr1"
2024-05-27T23:16:16.461+0300	INFO	gather	Gathering from cluster "dr2"
2024-05-27T23:16:18.624+0300	INFO	gather	Gathered 1441 resources from cluster "hub" in 2.163 seconds
2024-05-27T23:16:20.316+0300	INFO	gather	Gathered 1934 resources from cluster "dr2" in 3.855 seconds
2024-05-27T23:16:20.705+0300	INFO	gather	Gathered 1979 resources from cluster "dr1" in 4.244 seconds
2024-05-27T23:16:20.705+0300	INFO	gather	Gathered 5354 resources from 3 clusters in 4.245 seconds
```

This gathers 78 MiB of data into the directory "gather.all":

```
$ du -sh gather.all/
78M	gather.all/
```

The data compresses well and can be attached to a bug tracker:

```
$ tar czf gather.all.tar.gz gather.all
$ du -sh gather.all.tar.gz
6.4M	gather.all.tar.gz
```

The gather directory includes now all clusters:

```
$ tree -L 2 gather.all/
gather.all/
├── dr1
│   ├── addons
│   ├── cluster
│   └── namespaces
├── dr2
│   ├── addons
│   ├── cluster
│   └── namespaces
├── gather.log
└── hub
    ├── cluster
    └── namespaces
```

Clusters "dr1" and "dr2" have a "rook-ceph" storage system, so the
"rook" addon collected more data in the "addons" directory. The
"commands" directory contains output from various ceph commands, and the
"logs" directory contains external logs stored on the nodes. Since this
is a single node minikube cluster, we have only one node, "dr1".

```
$ tree gather.all/dr1/addons/rook/
gather.all/dr1/addons/rook/
├── commands
│   ├── ceph-osd-blocklist-ls
│   └── ceph-status
└── logs
    └── dr1
        ├── 59ccb238-dd08-4225-af2f-d9aef1ad4d29-client.rbd-mirror-peer.log
        ├── ceph-client.ceph-exporter.log
        ├── ceph-client.rbd-mirror.a.log
        ├── ceph-mds.myfs-a.log
        ├── ceph-mds.myfs-b.log
        ├── ceph-mgr.a.log
        ├── ceph-mon.a.log
        ├── ceph-osd.0.log
        └── ceph-volume.log
```

## Gathering data for specific namespaces

When debugging a problem, it is useful to gather data for specific
namespaces. This is very quick and produce a small amount of data.

Lets gather data from the "deployment-rbd" namespace. The namespace
exists on the "hub" and "dr1" clusters. Depending on the application
state, the namespace can be also on cluster "dr2".

To gather the "deployment-rbd" namespace from all clusters use:

```
$ kubectl gather --contexts hub,dr1,dr2 -n deployment-rbd -d gather.before
2024-05-27T23:33:45.883+0300	INFO	gather	Using kubeconfig "/home/nsoffer/.kube/config"
2024-05-27T23:33:45.888+0300	INFO	gather	Gathering from namespaces [deployment-rbd]
2024-05-27T23:33:45.888+0300	INFO	gather	Gathering from cluster "hub"
2024-05-27T23:33:45.888+0300	INFO	gather	Gathering from cluster "dr1"
2024-05-27T23:33:45.888+0300	INFO	gather	Gathering from cluster "dr2"
2024-05-27T23:33:45.905+0300	INFO	gather	Gathered 0 resources from cluster "dr2" in 0.017 seconds
2024-05-27T23:33:45.987+0300	INFO	gather	Gathered 18 resources from cluster "hub" in 0.099 seconds
2024-05-27T23:33:46.024+0300	INFO	gather	Gathered 24 resources from cluster "dr1" in 0.136 seconds
2024-05-27T23:33:46.024+0300	INFO	gather	Gathered 42 resources from 3 clusters in 0.137 seconds
```

This gathered tiny amount of data very quickly:

```
$ du -sh gather.before/
244K	gather.before/
```

This gathered everything under the specified namespace in all clusters
having this namespace:

```
$ tree -L 4 gather.before/
gather.before/
├── dr1
│   └── namespaces
│       └── deployment-rbd
│           ├── apps
│           ├── apps.open-cluster-management.io
│           ├── configmaps
│           ├── events.k8s.io
│           ├── persistentvolumeclaims
│           ├── pods
│           ├── ramendr.openshift.io
│           ├── replication.storage.openshift.io
│           └── serviceaccounts
├── gather.log
└── hub
    └── namespaces
        └── deployment-rbd
            ├── apps.open-cluster-management.io
            ├── cluster.open-cluster-management.io
            ├── configmaps
            ├── events.k8s.io
            ├── ramendr.openshift.io
            └── serviceaccounts
```

After failing over the application to cluster "dr2", we can gather the
data again to compare the application state before and after the
failover:

```
$ kubectl gather --contexts hub,dr1,dr2 -n deployment-rbd -d gather.after
2024-05-27T23:45:20.292+0300	INFO	gather	Using kubeconfig "/home/nsoffer/.kube/config"
2024-05-27T23:45:20.297+0300	INFO	gather	Gathering from namespaces [deployment-rbd]
2024-05-27T23:45:20.297+0300	INFO	gather	Gathering from cluster "hub"
2024-05-27T23:45:20.297+0300	INFO	gather	Gathering from cluster "dr1"
2024-05-27T23:45:20.297+0300	INFO	gather	Gathering from cluster "dr2"
2024-05-27T23:45:20.418+0300	INFO	gather	Gathered 23 resources from cluster "hub" in 0.121 seconds
2024-05-27T23:45:20.421+0300	INFO	gather	Gathered 20 resources from cluster "dr1" in 0.123 seconds
2024-05-27T23:45:20.435+0300	INFO	gather	Gathered 19 resources from cluster "dr2" in 0.137 seconds
2024-05-27T23:45:20.435+0300	INFO	gather	Gathered 62 resources from 3 clusters in 0.138 seconds
```

We can see that the application is running on cluster "dr2":

```
$ tree -L 4 gather.after/
gather.after/
├── dr1
│   └── namespaces
│       └── deployment-rbd
│           ├── configmaps
│           ├── events.k8s.io
│           └── serviceaccounts
├── dr2
│   └── namespaces
│       └── deployment-rbd
│           ├── apps
│           ├── apps.open-cluster-management.io
│           ├── configmaps
│           ├── events.k8s.io
│           ├── persistentvolumeclaims
│           ├── pods
│           ├── ramendr.openshift.io
│           ├── replication.storage.openshift.io
│           └── serviceaccounts
├── gather.log
└── hub
    └── namespaces
        └── deployment-rbd
            ├── apps.open-cluster-management.io
            ├── cluster.open-cluster-management.io
            ├── configmaps
            ├── events.k8s.io
            ├── ramendr.openshift.io
            └── serviceaccounts
```

Now we can compare application resource before and after the failover:

```
$ diff -u gather.before/hub/namespaces/deployment-rbd/ramendr.openshift.io/drplacementcontrols/deployment-rbd-drpc.yaml \
          gather.after/hub/namespaces/deployment-rbd/ramendr.openshift.io/drplacementcontrols/deployment-rbd-drpc.yaml
--- gather.before/hub/namespaces/deployment-rbd/ramendr.openshift.io/drplacementcontrols/deployment-rbd-drpc.yaml	2024-05-27 23:33:45.979547024 +0300
+++ gather.after/hub/namespaces/deployment-rbd/ramendr.openshift.io/drplacementcontrols/deployment-rbd-drpc.yaml	2024-05-27 23:45:20.405342350 +0300
@@ -3,13 +3,13 @@
 metadata:
   annotations:
     drplacementcontrol.ramendr.openshift.io/app-namespace: deployment-rbd
-    drplacementcontrol.ramendr.openshift.io/last-app-deployment-cluster: dr1
+    drplacementcontrol.ramendr.openshift.io/last-app-deployment-cluster: dr2
     kubectl.kubernetes.io/last-applied-configuration: |
...
```

## Gathering remote clusters

When gathering remote clusters it can be faster to gather the data on
the remote clusters and download the data to the local directory.

> [!IMPORTANT]
> Gathering remotely require the "oc" command.

In this example we gather data from OpenShift Data Foundation clusters
configured for disaster recovery. Gathering everything takes more than 6
minutes:

    $ kubectl gather --contexts kevin-rdr-hub,kevin-rdr-c1,kevin-rdr-c2 --remote --directory gather.remote
    2024-05-28T20:57:32.684+0300	INFO	gather	Using kubeconfig "/home/nsoffer/.kube/config"
    2024-05-28T20:57:32.686+0300	INFO	gather	Gathering from all namespaces
    2024-05-28T20:57:32.686+0300	INFO	gather	Gathering on remote cluster "kevin-rdr-c2"
    2024-05-28T20:57:32.686+0300	INFO	gather	Gathering on remote cluster "kevin-rdr-c1"
    2024-05-28T20:57:32.686+0300	INFO	gather	Gathering on remote cluster "kevin-rdr-hub"
    2024-05-28T20:59:49.362+0300	INFO	gather	Gathered on remote cluster "kevin-rdr-hub" in 136.676 seconds
    2024-05-28T21:02:45.090+0300	INFO	gather	Gathered on remote cluster "kevin-rdr-c2" in 312.404 seconds
    2024-05-28T21:03:51.841+0300	INFO	gather	Gathered on remote cluster "kevin-rdr-c1" in 379.155 seconds
    2024-05-28T21:03:51.841+0300	INFO	gather	Gathered 3 clusters in 379.155 seconds

This gathered 11 GiB of data:

```
$ du -sh gather.remote/
11G	gather.remote/
```

Most of the data is Ceph logs stored on the nodes:

```
$ du -sm gather.remote/*/*/*/* | sort -rn | head
2288	gather.remote/kevin-rdr-c1/quay-io-nirsof-gather-sha256-8999a022a9f243df3255f8bb41977fd6936c311cb20a015cbc632a873530da9e/addons/rook
2190	gather.remote/kevin-rdr-c2/quay-io-nirsof-gather-sha256-8999a022a9f243df3255f8bb41977fd6936c311cb20a015cbc632a873530da9e/addons/rook
583	gather.remote/kevin-rdr-c2/quay-io-nirsof-gather-sha256-8999a022a9f243df3255f8bb41977fd6936c311cb20a015cbc632a873530da9e/namespaces/openshift-storage
501	gather.remote/kevin-rdr-c1/quay-io-nirsof-gather-sha256-8999a022a9f243df3255f8bb41977fd6936c311cb20a015cbc632a873530da9e/namespaces/openshift-storage
282	gather.remote/kevin-rdr-hub/quay-io-nirsof-gather-sha256-8999a022a9f243df3255f8bb41977fd6936c311cb20a015cbc632a873530da9e/namespaces/openshift-openstack-infra
241	gather.remote/kevin-rdr-c1/quay-io-nirsof-gather-sha256-8999a022a9f243df3255f8bb41977fd6936c311cb20a015cbc632a873530da9e/namespaces/openshift-openstack-infra
232	gather.remote/kevin-rdr-c2/quay-io-nirsof-gather-sha256-8999a022a9f243df3255f8bb41977fd6936c311cb20a015cbc632a873530da9e/namespaces/openshift-ovn-kubernetes
189	gather.remote/kevin-rdr-c1/quay-io-nirsof-gather-sha256-8999a022a9f243df3255f8bb41977fd6936c311cb20a015cbc632a873530da9e/namespaces/openshift-monitoring
185	gather.remote/kevin-rdr-hub/quay-io-nirsof-gather-sha256-8999a022a9f243df3255f8bb41977fd6936c311cb20a015cbc632a873530da9e/namespaces/openshift-ovn-kubernetes
174	gather.remote/kevin-rdr-c2/quay-io-nirsof-gather-sha256-8999a022a9f243df3255f8bb41977fd6936c311cb20a015cbc632a873530da9e/namespaces/openshift-openstack-infra
```

Gathering only specific namespaces from these clusters is much quicker.
In this example we gather data related to single DR protected VM:

```
$ kubectl gather --contexts kevin-rdr-hub,kevin-rdr-c1,kevin-rdr-c2 --namespaces openshift-dr-ops,ui-vms3 --remote -d gather.remote.app
2024-05-28T21:14:15.883+0300	INFO	gather	Using kubeconfig "/home/nsoffer/.kube/config"
2024-05-28T21:14:15.884+0300	INFO	gather	Gathering from namespaces [openshift-dr-ops ui-vms3]
2024-05-28T21:14:15.884+0300	INFO	gather	Gathering on remote cluster "kevin-rdr-c2"
2024-05-28T21:14:15.884+0300	INFO	gather	Gathering on remote cluster "kevin-rdr-c1"
2024-05-28T21:14:15.884+0300	INFO	gather	Gathering on remote cluster "kevin-rdr-hub"
2024-05-28T21:14:33.247+0300	INFO	gather	Gathered on remote cluster "kevin-rdr-c2" in 17.363 seconds
2024-05-28T21:14:33.491+0300	INFO	gather	Gathered on remote cluster "kevin-rdr-c1" in 17.607 seconds
2024-05-28T21:14:33.577+0300	INFO	gather	Gathered on remote cluster "kevin-rdr-hub" in 17.692 seconds
2024-05-28T21:14:33.577+0300	INFO	gather	Gathered 3 clusters in 17.693 seconds
```

This gathers only 2.7 MiB:

```
$ du -sh gather.remote.app/
2.7M	gather.remote.app/
```

## Similar projects

- [must-gather](https://github.com/openshift/must-gather) - similar tool
for collecting data from OpenShift cluster.
- [SoS](https://github.com/sosreport/sos) - similar tool for collecting
data from a host.

## License

kubectl-gather is under the [Apache 2.0 license](/LICENSE)
