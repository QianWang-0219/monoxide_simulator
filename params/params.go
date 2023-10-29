package params

import (
	"sync"
)

// var BrokerList []string
var Ip_nodetable map[uint64]string
var TcpLock sync.Mutex

var LogWrite_path = "./log/"
var DataWrite_path = "./result/"

var Partition_shardnum = 4

// 模拟运行时间
var Generate_block int = 40
var Consensus int = 30
var On_blockchain int = 20
