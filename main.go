package main

import (
	"simple_go/monoxide/build"

	"github.com/spf13/pflag"
)

var (
	isClient bool
	shardID  int
)

func main() {
	pflag.BoolVarP(&isClient, "client", "c", false, "whether this node is a client")
	pflag.IntVarP(&shardID, "shardID", "s", 0, "id of the shard to which this node belongs, for example, 0")
	pflag.Parse()
	if isClient {
		build.BuildSupervisor()
	} else {
		build.BuildNewPbftNode(uint64(shardID))
	}
}
