package build

import (
	"simple_go/monoxide/params"
	"strconv"
)

const (
	BatchSize           int    = 16000 //it should be larger than inject speed
	TotalDataSize       int    = 1000000
	InjectSpeed         int    = 2500  //supervisor txsending发送的频率 %xx
	MaxBlockSize_global uint64 = 2000  //block size，一次打包交易的大小
	BlockInterval       int    = 10000 //10s生成一个区块（打包交易）
	ip_supervisor       string = "127.0.0.1:18800"
)

func BuildSupervisor() {
	params.Ip_nodetable = make(map[uint64]string)
	for i := uint64(0); i < uint64(params.Partition_shardnum); i++ {
		params.Ip_nodetable[i] = "127.0.0.1:" + strconv.Itoa(28800+int(i))
	}

	super := NewSupervisor()

	go super.SupervisorTxHandling() //读文件，发交易
	super.superListen()             //接受blockinfo
}

func BuildNewPbftNode(shardID uint64) {
	// pbft node
	params.Ip_nodetable = make(map[uint64]string)
	for i := uint64(0); i < uint64(params.Partition_shardnum); i++ {
		params.Ip_nodetable[i] = "127.0.0.1:" + strconv.Itoa(28800+int(i))
	}

	worker := NewNode(shardID)
	go worker.Propose()         //打包区块，模拟共识 inbox <-
	go worker.Reply()           //<-n.inbox 模拟共识完成,给supervior发送blockinfo
	go worker.ProcessMessages() //tcp监听supervisor消息，处理inject消息

	// 保持主程序不退出
	select {}
}
