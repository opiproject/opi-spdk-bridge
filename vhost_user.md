# SPDK vhost-user Target Overview

From <https://github.com/kata-containers/documentation/blob/master/use-cases/using-SPDK-vhostuser-and-kata.md>

The Storage Performance Development Kit (SPDK) provides a set of tools and
libraries for writing high performance, scalable, user-mode storage applications.

virtio, vhost and vhost-user:

## virtio

- virtio is an efficient way to transport data for virtual environments and
guests. It is most commonly used in QEMU VMs, where the VM itself exposes a
virtual PCI device and the guest OS communicates with it using a specific virtio
PCI driver. Its diagram is:
```
+---------+------+--------+----------+--+
|         +------+-------------------+  |
|         |            +----------+  |  |
| user    |            |          |  |  |
| space   |            |  guest   |  |  |
|         |            |          |  |  |
|    +----+ qemu       | +-+------+  |  |
|    |    |            | | virtio |  |  |
|    |    |            | | driver |  |  |
|    |    |            +-+---++---+  |  |
|    |    +------+-------------------+  |
|    |       ^               |          |
|    |       |               |          |
|    v       |               v          |
+-+------+---+------------+--+-------+--+
| |block |   +------------+ kvm.ko   |  |
| |device|                |          |  |
| +------+                +--+-------+  |
|           host kernel                 |
+---------------------------------------+
```

## vhost

- vhost is a protocol for devices accessible via inter-process communication. It
uses the same virtio queue layout as virtio to allow vhost devices to be mapped
directly to virtio devices. The initial vhost implementation is a part of the
Linux kernel and uses an ioctl interface to communicate with userspace
applications. Its diagram is:
```
+---------+------+--------+----------+--+
|         +------+-------------------+  |
|         |            +----------+  |  |
| user    |            |          |  |  |
| space   |            |  guest   |  |  |
|         |            |          |  |  |
|         | qemu       | +-+------+  |  |
|         |            | | virtio |  |  |
|         |            | | driver |  |  |
|         |            +-+-----++-+  |  |
|         +------+-------------------+  |
|                               |       |
|                               |       |
+-+------+--+-------------+--+--v-------+
| |block |  |vhost-scsi.ko|  | kvm.ko   |
| |device|  |             |  |          |
| +---^--+  +-v---------^-+  +--v-------+
|     |       |   host  |       |       |
|     +-------+  kernel +-------+       |
+---------------------------------------+
```

## vhost-user

- vhost-user implements the control plane through Unix domain socket to establish
virtio queue sharing with a user space process on the same host. SPDK exposes
vhost devices via the vhost-user protocol. Its diagram is:
```
+----------------+------+--+----------+-+
|                +------+-------------+ |
| user           |      +----------+  | |
| space          |      |          |  | |
|                |      |  guest   |  | |
|  +-+-------+   | qemu | +-+------+  | |
|  | vhost   |   |      | | virtio |  | |
|  | backend |   |      | | driver |  | |
|  +-^-^---^-+   |      +-+--+-----+  | |
|    | |   |     |           |        | |
|    | |   |     +--+---+----V------+-+ |
|    | |   |        |        |      |   |
|    | |  ++--------+--+     |      |   |
|    | |  |unix sockets|     |      |   |
|    | |  +------------+     |      |   |
|    | |                     |      |   |
|    | |  +-------------+    |      |   |
|    | +--|shared memory|<---+      |   |
+----+----+-------------+---+--+----+---+
|    |                      |           |
|    +----------------------+ kvm.ko    |
|                           +--+--------+
|           host kernel                 |
+---------------------------------------+
```

SPDK vhost is a vhost-user slave server. It exposes Unix domain sockets and
allows external applications to connect. It is capable of exposing virtualized
storage devices to QEMU instances or other arbitrary processes.

Currently, the SPDK vhost-user target can exposes these types of virtualized
devices:

- `vhost-user-blk`
- `vhost-user-scsi`
- `vhost-user-nvme`

For more information, visit [SPDK](https://spdk.io) and [SPDK vhost-user target](https://spdk.io/doc/vhost.html).
