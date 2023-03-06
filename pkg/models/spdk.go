// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.

// Package models holds definitions for SPDK json RPC structs
package models

// AccelCryptoKeyCreateParams holds the parameters required to create an AIO Block Device
type AccelCryptoKeyCreateParams struct {
	Cipher string `json:"cipher"`
	Key    string `json:"key"`
	Key2   string `json:"key2"`
	Name   string `json:"name"`
}

// AccelCryptoKeyCreateResult is the result of deleting an AIO Block Device
type AccelCryptoKeyCreateResult bool

// AccelCryptoKeyDestroyParams holds the parameters required to create an AIO Block Device
type AccelCryptoKeyDestroyParams struct {
	KeyName string `json:"key_name"`
}

// AccelCryptoKeyDestroyResult is the result of deleting an AIO Block Device
type AccelCryptoKeyDestroyResult bool

// GetVersionResult is the result of getting a version
type GetVersionResult struct {
	Version string `json:"version"`
	Fields  struct {
		Major  int    `json:"major"`
		Minor  int    `json:"minor"`
		Patch  int    `json:"patch"`
		Suffix string `json:"suffix"`
	} `json:"fields"`
}

// BdevAioCreateParams holds the parameters required to create an AIO Block Device
type BdevAioCreateParams struct {
	Name      string `json:"name"`
	Filename  string `json:"filename"`
	BlockSize int    `json:"block_size"`
}

// BdevAioCreateResult is the result of creating an AIO Block Device
type BdevAioCreateResult string

// BdevAioDeleteParams holds the parameters required to delete an AIO Block Device
type BdevAioDeleteParams struct {
	Name string `json:"name"`
}

// BdevAioDeleteResult is the result of deleting an AIO Block Device
type BdevAioDeleteResult bool

// BdevMalloCreateParams holds the parameters required to create a Malloc Block Device
type BdevMalloCreateParams struct {
	NumBlocks int    `json:"num_blocks"`
	BlockSize int    `json:"block_size"`
	Name      string `json:"name"`
	UUID      string `json:"uuid"`
}

// BdevAMalloCreateResult is the result of creating a Malloc Block Device
type BdevAMalloCreateResult string

// BdevMallocDeleteParams holds the parameters required to delete a Malloc Block Device
type BdevMallocDeleteParams struct {
	Name string `json:"name"`
}

// BdevMallocDeleteResult is the result of deleting a Malloc Block Device
type BdevMallocDeleteResult bool

// BdevNullCreateParams holds the parameters required to create a Null Block Device
// that discards all writes and returns undefined data for reads
type BdevNullCreateParams struct {
	BlockSize int    `json:"block_size"`
	NumBlocks int    `json:"num_blocks"`
	Name      string `json:"name"`
}

// BdevNullCreateResult is the result of creating a Null Block Device
type BdevNullCreateResult string

// BdevNullDeleteParams holds the parameters required to delete a Null Block Device
type BdevNullDeleteParams struct {
	Name string `json:"name"`
}

// BdevNullDeleteResult is the result of deleting a Null Block Device
type BdevNullDeleteResult bool

// BdevCryptoCreateParams holds the parameters required to create a Crypto Block Device
type BdevCryptoCreateParams struct {
	BaseBdevName string `json:"base_bdev_name"`
	Name         string `json:"name"`
	KeyName      string `json:"key_name"`
}

// BdevCryptoCreateResult is the result of creating a Crypto Block Device
type BdevCryptoCreateResult string

// BdevCryptoDeleteParams holds the parameters required to delete a Crypto Block Device
type BdevCryptoDeleteParams struct {
	Name string `json:"name"`
}

// BdevCryptoDeleteResult is the result of deleting a Crypto Block Device
type BdevCryptoDeleteResult bool

// BdevNvmeAttachControllerParams is the parameters required to create a block device based on an NVMe device
type BdevNvmeAttachControllerParams struct {
	Name    string `json:"name"`
	Trtype  string `json:"trtype"`
	Traddr  string `json:"traddr"`
	Hostnqn string `json:"hostnqn"`
	Adrfam  string `json:"adrfam"`
	Trsvcid string `json:"trsvcid"`
	Subnqn  string `json:"subnqn"`
}

// BdevNvmeAttachControllerResult is the result of creating a block device based on an NVMe device
type BdevNvmeAttachControllerResult string

// BdevNvmeDetachControllerParams is the parameters required to detach a block device based on an NVMe device
type BdevNvmeDetachControllerParams struct {
	Name string `json:"name"`
}

// BdevNvmeDetachControllerResult is the result of detaching a block device based on an NVMe device
type BdevNvmeDetachControllerResult bool

// BdevNvmeGetControllerParams is the parameters required to get a block device based on an NVMe device
type BdevNvmeGetControllerParams struct {
	Name string `json:"name"`
}

// BdevNvmeGetControllerResult is the result of getting a block device based on an NVMe device
type BdevNvmeGetControllerResult struct {
	Name   string `json:"name"`
	Ctrlrs []struct {
		State string `json:"state"`
		Trid  struct {
			Trtype  string `json:"trtype"`
			Adrfam  string `json:"adrfam"`
			Traddr  string `json:"traddr"`
			Trsvcid string `json:"trsvcid"`
			Subnqn  string `json:"subnqn"`
		} `json:"trid"`
		Cntlid int `json:"cntlid"`
		Host   struct {
			Nqn   string `json:"nqn"`
			Addr  string `json:"addr"`
			Svcid string `json:"svcid"`
		} `json:"host"`
	} `json:"ctrlrs"`
}

// BdevGetBdevsParams is the parameters required to get a block device
type BdevGetBdevsParams struct {
	Name string `json:"name"`
}

// BdevGetBdevsResult is the result of getting a block device
type BdevGetBdevsResult struct {
	Name      string `json:"name"`
	BlockSize int64  `json:"block_size"`
	NumBlocks int64  `json:"num_blocks"`
	UUID      string `json:"uuid"`
}

// BdevGetIostatParams hold the parameters required to get the IO stats of a block device
type BdevGetIostatParams struct {
	Name string `json:"name"`
}

// BdevGetIostatResult hold the results of getting the IO stats of a block device
type BdevGetIostatResult struct {
	TickRate int   `json:"tick_rate"`
	Ticks    int64 `json:"ticks"`
	Bdevs    []struct {
		Name              string `json:"name"`
		BytesRead         int    `json:"bytes_read"`
		NumReadOps        int    `json:"num_read_ops"`
		BytesWritten      int    `json:"bytes_written"`
		NumWriteOps       int    `json:"num_write_ops"`
		BytesUnmapped     int    `json:"bytes_unmapped"`
		NumUnmapOps       int    `json:"num_unmap_ops"`
		ReadLatencyTicks  int    `json:"read_latency_ticks"`
		WriteLatencyTicks int    `json:"write_latency_ticks"`
		UnmapLatencyTicks int    `json:"unmap_latency_ticks"`
	} `json:"bdevs"`
}

// VhostCreateBlkControllerParams holds the parameters required to create a block device
// from a vhost controller
type VhostCreateBlkControllerParams struct {
	Ctrlr   string `json:"ctrlr"`
	DevName string `json:"dev_name"`
}

// VhostCreateBlkControllerResult is the result of creating a block device from a vhost controller
type VhostCreateBlkControllerResult bool

// VhostDeleteControllerParams holds the parameters required to delete a vhost controller
type VhostDeleteControllerParams struct {
	Ctrlr string `json:"ctrlr"`
}

// VhostDeleteControllerResult is the result of deleting a vhost controller
type VhostDeleteControllerResult bool

// VhostGetControllersParams holds the parameters required to get a vhost controller
type VhostGetControllersParams struct {
	Name string `json:"name"`
}

// VhostGetControllersResult is the result of getting a vhost controller
type VhostGetControllersResult struct {
	Ctrlr           string `json:"ctrlr"`
	Cpumask         string `json:"cpumask"`
	DelayBaseUs     int    `json:"delay_base_us"`
	IopsThreshold   int    `json:"iops_threshold"`
	Socket          string `json:"socket"`
	BackendSpecific struct {
		Block struct {
			Readonly bool   `json:"readonly"`
			Bdev     string `json:"bdev"`
		} `json:"block"`
	} `json:"backend_specific"`
}

// VhostCreateScsiControllerParams holds the parameters required to create a SCSI controller
type VhostCreateScsiControllerParams struct {
	Ctrlr string `json:"ctrlr"`
}

// VhostCreateScsiControllerResult is the result of creating a SCSI controller
type VhostCreateScsiControllerResult bool

// NvmfSubsystemAddNsParams holds the parameters required to add a namespace to an existing subsystem
type NvmfSubsystemAddNsParams struct {
	Nqn       string `json:"nqn"`
	Namespace struct {
		Nsid     int    `json:"nsid"`
		BdevName string `json:"bdev_name"`
	} `json:"namespace"`
}

// NvmfSubsystemAddNsResult is the result NSID of attaching a namespace to an existing subsystem
type NvmfSubsystemAddNsResult int

// NvmfSubsystemRemoveNsParams holds the parameters required to Delete a NVMf subsystem
type NvmfSubsystemRemoveNsParams struct {
	Nqn  string `json:"nqn"`
	Nsid int    `json:"nsid"`
}

// NvmfSubsystemRemoveNsResult is the result of creating a NVMf subsystem
type NvmfSubsystemRemoveNsResult bool

// NvmfCreateSubsystemParams holds the parameters required to create a NVMf subsystem
type NvmfCreateSubsystemParams struct {
	Nqn           string `json:"nqn"`
	SerialNumber  string `json:"serial_number"`
	ModelNumber   string `json:"model_number"`
	AllowAnyHost  bool   `json:"allow_any_host"`
	MaxNamespaces int    `json:"max_namespaces"`
}

// NvmfCreateSubsystemResult is the result of creating a NVMf subsystem
type NvmfCreateSubsystemResult bool

// NvmfDeleteSubsystemParams holds the parameters required to Delete a NVMf subsystem
type NvmfDeleteSubsystemParams struct {
	Nqn string `json:"nqn"`
}

// NvmfDeleteSubsystemResult is the result of creating a NVMf subsystem
type NvmfDeleteSubsystemResult bool

// NvmfGetSubsystemsResult is the result of listing all NVMf subsystems
type NvmfGetSubsystemsResult struct {
	Nqn             string        `json:"nqn"`
	Subtype         string        `json:"subtype"`
	ListenAddresses []interface{} `json:"listen_addresses"`
	AllowAnyHost    bool          `json:"allow_any_host"`
	Hosts           []interface{} `json:"hosts"`
	SerialNumber    string        `json:"serial_number,omitempty"`
	ModelNumber     string        `json:"model_number,omitempty"`
	MaxNamespaces   int           `json:"max_namespaces,omitempty"`
	MinCntlid       int           `json:"min_cntlid,omitempty"`
	MaxCntlid       int           `json:"max_cntlid,omitempty"`
	Namespaces      []struct {
		Nsid int    `json:"nsid"`
		Name string `json:"name"`
	} `json:"namespaces,omitempty"`
}

// NvmfGetSubsystemStatsResult is the result of NVMf subsystem statistics
type NvmfGetSubsystemStatsResult struct {
	TickRate   int `json:"tick_rate"`
	PollGroups []struct {
		Name               string `json:"name"`
		AdminQpairs        int    `json:"admin_qpairs"`
		IoQpairs           int    `json:"io_qpairs"`
		CurrentAdminQpairs int    `json:"current_admin_qpairs"`
		CurrentIoQpairs    int    `json:"current_io_qpairs"`
		PendingBdevIo      int    `json:"pending_bdev_io"`
		Transports         []struct {
			Trtype string `json:"trtype"`
		} `json:"transports"`
	} `json:"poll_groups"`
}

// NvmfSubsystemAddListenerParams holds the parameters required to Delete a NVMf subsystem
type NvmfSubsystemAddListenerParams struct {
	Nqn           string `json:"nqn"`
	ListenAddress struct {
		Trtype  string `json:"trtype"`
		Traddr  string `json:"traddr"`
		Trsvcid string `json:"trsvcid,omitempty"`
		Adrfam  string `json:"adrfam,omitempty"`
	} `json:"listen_address"`
}

// NvmfSubsystemAddListenerResult is the result of creating a NVMf subsystem
type NvmfSubsystemAddListenerResult bool
