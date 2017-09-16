Dory
===
# Kubernetes Flexvolume Driver for Docker Volume Plugins

Dory is a driver for the [Kubernetes Flexvolume](https://github.com/kubernetes/community/blob/master/contributors/devel/flexvolume.md) Volume type. This driver translates Flexvolume requests to [Docker Volume Plugin](https://docs.docker.com/engine/extend/plugins_volume/) requests. This allows the administrator to leverage [existing Docker Volume Plugins](https://docs.docker.com/engine/extend/legacy_plugins/) in a Kubernetes cluster. Dory provides the ability to 'just in time' provision storage as well as have the orchestrator automatically attach/mount and detach/unmount Persistent Volumes.

Why is this called Dory? Because [Dory speaks whale](https://www.google.com/search?q=Dory+speaks+whale).

What about the [Container Storage Interface](https://github.com/container-storage-interface/)? The CSI is certainly the future for container storage. Dory provides a stop gap while the CSI specification is ratified, orchestrators begin supporting it, and implementations begin to surface.

# Design
## Overview
In order to provide persistent storage for a Pod, Kubernetes makes a call to Dory via the Flexvolume interface. Dory is an executable placed in a specific directory (see [Installation](#installation)). Dory then communicates with the configured Docker Volume Plugin via a unix socket. Dory then translates the response from the Docker Volume Plugin to the Flexvolume interface. It is important to note that Dory never communicates with the container engine, it communicates directly with the Docker Volume Plugin. This theoretically allows Dory to work with any container engine supported by Kubernetes.

## Create
Dory is configured by default to create a volume if one with that name doesn't exist. It does this by first using the Docker Volume Plugin 'get' function. If this doesn't return a volume (and Dory is configured to create volumes), Dory will call the Docker Volume Plugin 'create' function using the options specified in the Persistent Volume definition. This is handled during the Attach workflow in Kubernetes 1.5 and in the Mount workflow in 1.6 and higher.

## Mount
The diagram below depicts the process communication on the right and the resulting objects on the left. When the Mount workflow is executed Dory first uses the Docker Volume Plugin 'get' function to see if the volume is available (see [Create](#create)). It then executes the Docker Volume Plugin 'mount' function to mount the filesystem. The Pod uuid is used as the Docker Volume Plugin 'mount id'. This results in the green cylinder labeled '/vol/HrPostgres' in the diagram. Dory then bind mounts the path returned by the Docker Volume Plugin to the location that Kubernetes has requested. This results in the dark blue cylinder in the diagram. If SELinux is configured on the kubelet Dory will set the proper context for this mount.

<img src="mount.png">

## Unmount
The unmount workflow unmounts the bind mount and then uses the Docker Volume Plugin 'unmount' function to unmount and detach the filesystem from the kubelet.

# Building Dory
Dory is written in Go and requires golang on your machine. The following example installs the necessary tools and builds Dory on a RHEL 7.4 system:
```
$ sudo subscription-manager repos --enable=rhel-7-server-optional-rpms
$ sudo yum install -y golang make
$ git clone https://github.com/hpe-storage/dory
$ make gettools
$ make dory
```
You should end up with a `dory` executable in the `./bin` directory and be ready for [installation](#installation).

**Hint:** Go is available through the [EPEL](https://fedoraproject.org/wiki/EPEL) repository for .rpm based distributions and a `golang` package is part of the official Ubuntu repositories.

# Usage

## Installation
Create a directory on each kubelet with using the following convention: `/usr/libexec/kubernetes/kubelet-plugins/volume/exec/dory~plugin` where `plugin` is replaced with the name of the Docker Volume Plugin. Then copy the dory binary to this folder naming the file to the name of the Docker Volume Plugin. For example, in order to use [HPE's Nimble Storage Docker Volume Plugin](https://connect.nimblestorage.com/community/app-integration/docker), the following directory should be created: `/usr/libexec/kubernetes/kubelet-plugins/volume/exec/dory~nimble`. The Dory binary should be copied to `/usr/libexec/kubernetes/kubelet-plugins/volume/exec/dory~nimble/nimble`.

## Configuration
Dory looks for a configuration file with the same name as the executable with a `.json` extension. Following the example above, the configuration file would be `/usr/libexec/kubernetes/kubelet-plugins/volume/exec/dory~nimble/nimble.json`.

### Docker Volume Plugin Socket Path
The critical attribute in this file is called `"dockerVolumePluginSocketPath"`. This tells Dory where the Docker Volume Plugin socket file is. Again, following the example above, the file would contain the following;
```
{
    "dockerVolumePluginSocketPath": "/run/docker/plugins/nimble.sock"
}
```

### Logging
There are two attributes which control how Dory logs. The `"logFilePath"` attribute provides the full path to the log file. The `"logDebug"` indicates whether to log at a debug granularity. The following are the default values;
```
{
...
    "logFilePath": "/var/log/dory.log",
    "logDebug": false
}
```

### Behavior
There are two attributes which control Dory's behavior. The `"createVolumes"` attribute indicates whether Dory should create a volume when it can't find one. The `"stripK8sFromOptions"` attribute indicates whether the options in the Kubernetes.io namespace should be passed on to the Docker Volume Driver. The following are the default values;
```
{
...
    "stripK8sFromOptions": true,
    "createVolumes": true
}
```

### Example
The following is an example of the default values;
```
{
    "dockerVolumePluginSocketPath": "/run/docker/plugins/nimble.sock",
    "logFilePath": "/var/log/dory.log",
    "logDebug": false,
    "stripK8sFromOptions": true,
    "createVolumes": true
}
```

## What's in a name?
There are several names that you should be aware of when using Dory. The first is the Docker Volume name. This is used by Dory to identify the Docker Volume that should be exposed to Kubernetes. The second name to be aware of is that of the Persistent Volume. This name is used by Kubernetes to identify the Persistent Volume object (for example, in the output of `kubectl get pv`). The final name to be aware of is that of the Persistent Volume Claim. This name is used to tie the claim to a Pod or Pod template.

Each of these names can be different. Some administrators follow a naming pattern in order to easily identify these relationships. For example, when using a Docker Volume named "sqldata" they might create a Persistent Volume named "sqldata-pv" and a Persistent Volume Claim named "sqldata-pvc". The Pod definition would then reference "sqldata-pvc".

Giving the Docker Volume, Persistent Volume and Persistent Volume Claim all different names is not required though. Because these are each different objects, they can all share the same name. This makes it even easier to identify the relationship between these objects.

**Note:** In Kubernetes 1.5 and 1.6 Flexvolume didn't provide the Persistent Volume name to its driver, so the Docker Volume name must be provided an an option in the Persistent Volume definition. As of 1.7 the Persistent Volume name is passed to the driver.

## Sometimes size does matter
Flexvolume currently doesn't communicate the size of the volume to its driver. If the Docker Volume Plugin you're using requires a size, you'll need to specify this in the options section of the Persistent Volumes.

### Example
This example uses the Docker Volume Driver from [HPE's Nimble Storage](https://connect.nimblestorage.com/community/app-integration/docker) to create a Persistent Volume to back a MySQL database instance. First, the administrator creates a Persistent Volume named "sqldata-pv". In order to be able to reference this Persistent Volume explicitly in the claim, the 'volumeName.dory' label is added. The options section may contain any create options the Docker Volume Driver supports. The administrator then creates a Persistent Volume Claim which uses a matchLabels selector to find the Persistent Volume. Finally, the administrator creates a Replication Controller that references the claim by name. When the Replication Controller is created, the Persistent Volume is matched with the Persistent Volume Claim and the Docker Volume is then created and then mounted.

<img src="example.png">

# Future
What's next? We're looking extending this project to include a [Dynamic Provisioner](http://blog.kubernetes.io/2017/03/dynamic-provisioning-and-storage-classes-kubernetes.html). We're also hoping to add Windows support.

# Thanks
Thank you to [Chakravarthy Nelluri](https://github.com/chakri-nelluri) for all his work on [Flexvolume](https://github.com/kubernetes/kubernetes/commit/fa76de79e5d1670b8e6add30f0159c833534a298#diff-af00671c74d885ce20891c24516198e8) which has made this 'out of tree' work possible.

Thank you to [Michael Mattsson](https://connect.nimblestorage.com/people/mmattsson), TME extraordinaire, for his help testing and for writing a [blog post](https://connect.nimblestorage.com/community/app-integration/blog/2017/06/21/tech-preview-bringing-nimble-storage-to-kubernetes-and-openshift) about what we were up to.

# Licensing
Dory is licensed under the Apache License, Version 2.0. Please see [LICENSE](LICENSE) for the full license text.

## Vendoring
The only dependency is Nate Finch's lumberjack (MIT License). The import is still `gopkg.in/natefinch/lumberjack.v2`, but the actual location is the v2 branch of https://github.com/natefinch/lumberjack. The git commit id at time of copy was dd45e6a67c53f673bb49ca8a001fd3a63ceb640e.
