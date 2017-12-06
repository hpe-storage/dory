# Kubernetes Flexvolume Driver and StorageClass Provisioner for Docker Volume Plugins
Repository for Dory and Doryd: The FlexVolume driver and StorageClass provisioner for Kubernetes using *any* Docker Volume API compatible plugin. This is [Open Source Software](LICENSE) from [HPE DEV](https://developer.hpe.com).

# Dory
Dory is a driver for the [Kubernetes FlexVolume](https://github.com/kubernetes/community/blob/master/contributors/devel/flexvolume.md) Volume type. This driver translates Flexvolume requests to [Docker Volume Plugin](https://docs.docker.com/engine/extend/plugins_volume/) requests. This allows the administrator to leverage [existing legacy Docker Volume Plugins](https://docs.docker.com/engine/extend/legacy_plugins/) or [existing managed Docker Volume Plugins](https://store.docker.com/search?category=volume&q=&type=plugin) in a Kubernetes cluster. Managed plugins require Docker 1.13. Dory provides the ability to 'just in time' provision storage as well as have the orchestrator automatically attach/mount and detach/unmount Persistent Volumes.

* Dory [documentation](docs/dory/README.md)
* Binary releases:
  * [Master](http://dl.bintray.com/hpe-storage/dory/dory-master) (latest)
  * [1.0](http://dl.bintray.com/hpe-storage/dory/dory-1.0)

# Doryd
Doryd is a implementation of the [Out-of-tree provisioner](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/storage/volume-provisioning.md) that dynamically provisions persistent storage using the [Docker Volume Plugin API](https://docs.docker.com/engine/extend/plugins_volume/).

* Doryd [documentation](docs/doryd/README.md)
* Binary releases:
  * [Master](http://dl.bintray.com/hpe-storage/dory/doryd-master) (latest)
* Container image:
  * [doryd](https://hub.docker.com/r/nimblestorage/doryd/)

# Plugins
To better help end-users navigate around the storage landcape we've composed a page to help keep track of what Docker Volume plugins are known to work well and some record keeping on issues/gotchas with the specific plugin.

* [Plugins known to work](docs/plugins/README.md)

# Project
Why is the project called Dory? Because [Dory speaks whale](https://www.google.com/search?q=Dory+speaks+whale).

What about the [Container Storage Interface](https://github.com/container-storage-interface/)? The CSI is certainly the future for container storage. Dory provides a stop gap while the CSI specification is ratified, orchestrators begin supporting it, and implementations begin to surface.

# Thanks
Thank you to [Chakravarthy Nelluri](https://github.com/chakri-nelluri) for all his work on [Flexvolume](https://github.com/kubernetes/kubernetes/commit/fa76de79e5d1670b8e6add30f0159c833534a298#diff-af00671c74d885ce20891c24516198e8) which has made this 'out of tree' work possible.

Thank you to [Michael Mattsson](https://community.hpe.com/t5/user/viewprofilepage/user-id/1879662), TME extraordinaire, for his help testing and for writing a [blog post](https://community.hpe.com/t5/HPE-Nimble-Storage-Tech-Blog/Tech-Preview-Bringing-Nimble-Storage-to-Kubernetes-and-OpenShift/ba-p/6986748) about what we were up to.

# Licensing
Dory and Doryd is licensed under the Apache License, Version 2.0. Please see [LICENSE](LICENSE) for the full license text.
