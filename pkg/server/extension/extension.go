// Package extension provides capabilities to use extension to change base SPDK
// server behavior
// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation
package extension

import (
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
)

/*
This package provides the ability to create an extension for xPU by exposing
default SPDK JSON RPC interface, but requiring adjustments only for some of
the calls.

There are several ways how to add an extension:
- By forking this repo and providing the extension code in the fork's
pkg/server/extension package.
- By importing opi-spdk-bridge package into a dedicated xPU repo and using this
extension package to register your addition.

To provide an extension, a user needs to create a dedicated struct which is
leveraging Go embedding capabilities for server.OpiServer interface e.g.
```

	type myXpuServer struct  {
		server.OpiServer
		muField1 string
		muField2 string
	}

```

After that customizations should be provided for required calls e.g.
```

	func (s *myXpuServer) CreateVirtioBlk(
		ctx context.Context,
		in *pb.CreateVirtioBlkRequest) (*pb.VirtioBlk, error) {

		// do something specific for this xPU
	}

```

Here, the rest of the calls rely on the default opi-spdk-bridge implementation,
but CreateVirtioBlk will have its own behavior.

Then a function to embed the base SPDK bridge into myXpuServer should be
created.
```

	func myExtension(baseSpdkServer server.OpiServer) (server.OpiServer, error) {
		return myXpuServer {
			baseSpdkServer, // embed the default bridge
			"my param 1",
			"my param 2"
		}, nil
	}

```

The function above should be passed into RegisterExtension function.
- If extension code was added into extension package, then init() function can
be used to register the extension.
```
	// somewhere in extension directory
	func init() {
		RegisterExtension(myExtension)
	}

```
- If opi-spdk-bridge server was imported into a dedicated xPU repository as a
package, then a separate main should be added with extension registration
and the bridge needs starting like:
```

	// myXpuMain.go
	import github.com/opiproject/opi-spdk-bridge/cmd/server
	import github.com/opiproject/opi-spdk-bridge/pkg/server/extension

	func main() {
		extension.RegisterExtension(myExtension)
		server.Run()
	}

```
*/

// Extension type declares function signature which can be used
// to apply an extension to baseSpdkServer and return the extended server.
type Extension func(baseSpdkServer server.OpiServer) (server.OpiServer, error)

var extensions []Extension

// RegisterExtension is used to register a new Extension to be applied to the
// base SPDK server.
func RegisterExtension(extension Extension) {
	extensions = append(extensions, extension)
}

// Extend is used at the bridge start procedure to apply any registered
// extensions on baseSpdkServer and return the extended server.
func Extend(baseSpdkServer server.OpiServer) (server.OpiServer, error) {
	for _, extension := range extensions {
		s, err := extension(baseSpdkServer)
		if err != nil {
			return nil, err
		}
		baseSpdkServer = s
	}
	return baseSpdkServer, nil
}
