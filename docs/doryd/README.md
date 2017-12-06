# Doryd
Doryd is an out-of-tree dynamic provisioner for Docker Volume plugins that uses the StorageClass interface available in Kubernetes. Doryd needs to run on every node in the cluster where you would normally schedule workloads that require persistent storage provisioned by Doryd. Doryd also depends on [Dory](../dory/README.md), the FlexVolume driver for Docker Volume plugins.

# Building (optional)
Dory is written in Go and requires golang on your machine. The following example installs the necessary tools and builds Dory on a RHEL 7.4 system:
```
sudo subscription-manager repos --enable=rhel-7-server-optional-rpms
sudo yum install -y golang make
git clone https://github.com/hpe-storage/dory.git
make gettools
make vendor
make doryd
```

You should end up with a `doryd` executable in the `./bin` directory. A `Dockerfile` is not yet available in the repository but is fairly straight-forward:
```
FROM centos:latest
ADD [ "bin/doryd", "/usr/local/bin/doryd" ]
ENTRYPOINT [ "doryd" ]
CMD [ "/etc/kubernetes/admin.conf", "dev.hpe.com" ]
```

Build it with:
```
docker build -t doryd:latest .
```

**Hint:** Go is available through the [EPEL](https://fedoraproject.org/wiki/EPEL) repository for .rpm based distributions and a `golang` package is part of the official Ubuntu repositories.

# Running
Doryd is available on Docker Hub and an [example DaemonSet specification](../../examples/ds-doryd.yaml) is available.

## Prerequisities
The `doryd` binary needs access to the cluster via a kubeconfig file. The location may vary between distributions. The stock DaemonSet spec will assume the container default of `/etc/kubernetes/admin.conf`. This file needs to exist on all nodes prior to deploying the DaemonSet.

The default provisioner name is prefixed with `dev.hpe.com` and will listen for Persistent Volume Claims that asks for Storage Classes with `provisioner: dev.hpe.com/DockerVolumeDriverName` and will map against a [Dory FlexVolume driver](../dory/README.md) with the same `provisioner`. Hence it's important that the FlexVolume driver name matches up with what you name your provisioner.

A custom `doryd` command line could look like this:
```
doryd /root/.kube/cluster.conf nimblestorage.com
```

There should then be a Dory FlexVolume driver named `nimblestorage.com/yourdrivername` and Storage Classes should use `provisioner: nimblestorage.com/yourdrivername`.

## kubectl
Deploying the default DaemonSet out-of-the-box can be accomplished with:
```
kubectl apply -f https://raw.githubusercontent.com/hpe-storage/dory/master/examples/ds-doryd.yaml
```

Add arguments to the container image if using a custom path to kubeconfig or need a different prefix.

# Using
The [Kubernetes documentation](https://kubernetes.io/docs/concepts/storage/volumes/) is good source for learning about Storage Classes, Proivsioners, Persistent Volume Claims and Persistent Volumes. The following basic examples assumes familiarity with those basic concepts. A full tutorial for Dory and Doryd is available on [developer.hpe.com](https://developer.hpe.com/platform/nimble-storage/home)

Since Dory and Doryd solely relies on underlying Docker Volume plugin capabilities to provision storage, please consult the documentation for the corresponding plugin documentation.

The following example will use the `dev.hpe.com/nimble` FlexVolume driver to provision volumes capped at 5000 IOPS using a database optimized Performance Profile with a custom Protection Template that replicate data offsite.

```
kubectl create -f- <<EOF
---
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
 name: database
provisioner: dev.hpe.com/nimble
parameters:
  description: "Volume provisioned by doryd from database StorageClass"
  limitIOPS: "5000"
  perfPolicy: "SQL Server"
  protectionTemplate: "Retain-90Local-360Remote"
EOF
```

An end-user may then reference the Storage Class when creating a Persistent Volume Claim: 
```
kubectl create -f- <<EOF
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: myclaim
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 500Gi
  storageClassName: database
EOF
```

The key here is that the end-user have no interest in knowing any underlying storage terminology. The admin may change the entire Storage Class and backend vendor without breakage for the end-user. 

# Licensing
Doryd is licensed under the Apache License, Version 2.0. Please see [LICENSE](../../LICENSE) for the full license text.
