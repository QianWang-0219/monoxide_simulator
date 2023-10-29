package build

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"simple_go/monoxide/networks"
	"simple_go/monoxide/nlog"
	"simple_go/monoxide/params"
	"simple_go/monoxide/partition"
	"simple_go/monoxide/utils"
	"strconv"
	"sync"
	"time"
)

type Node struct {
	sid          uint64
	Txpool       *partition.TxPool
	requestPool  map[string]*partition.Request
	requested    chan []byte
	inbox        chan []byte
	sequenceID   uint64
	PartitionMap map[string]uint64
	pmlock       sync.RWMutex
	nl           *nlog.PbftLog
}

func NewNode(id uint64) *Node {
	return &Node{
		sid:          id,
		Txpool:       partition.NewTxPool(),
		requestPool:  make(map[string]*partition.Request),
		requested:    make(chan []byte),
		inbox:        make(chan []byte),
		nl:           nlog.NewPbftLog(id),
		PartitionMap: make(map[string]uint64),
		sequenceID:   0,
	}
}

func (n *Node) Propose() {
	req := make(chan []byte)
	for {
		// 打包交易 ，生成区块、request，可以等BlockInterval
		time.Sleep(time.Duration(int64(BlockInterval)) * time.Millisecond)

		// 把交易从缓冲池中删除（打包交易）
		packedTxs := n.Txpool.PackTxs(MaxBlockSize_global)

		n.nl.Plog.Println("node:", n.sid, "打包交易长度", len(packedTxs))
		time.Sleep(time.Duration(int64(params.Generate_block)) * time.Millisecond)

		// 生成request消息
		r := &partition.Request{
			Msg:     partition.Encode(packedTxs), //相当于block
			ReqTime: time.Now(),
		}
		digest := partition.GetDigest(r)
		//n.nl.Plog.Println("（主节点）生成request消息 ", r.ReqTime)

		// 把request的摘要放进请求池
		n.requestPool[string(digest)] = r
		go n.subThread(req, n.inbox)
		req <- digest
	}
}

func (n *Node) subThread(req chan []byte, inbox chan []byte) {
	// 生成preprepare消息广播给副本节点
	// ……开始共识过程
	for {
		select {
		case Digest := <-req:
			//n.nl.Plog.Println("（子协程）收到request摘要，模拟共识....")
			seqid := n.sequenceID
			// Simulate some processing time
			time.Sleep(time.Duration(params.Consensus) * time.Millisecond)
			// 构建commit消息
			c := partition.Commit{
				Digest: Digest,
				SeqID:  seqid,
			}
			//n.nl.Plog.Println("（子协程）模拟共识完成，构建commit消息", c)
			commitByte, err := json.Marshal(c)
			if err != nil {
				log.Panic()
			}
			msg_send := partition.MergeMessage(partition.CCommit, commitByte)
			inbox <- msg_send

		}
	}

}

func (n *Node) Reply() {
	for {
		select {
		case message := <-n.inbox:
			msgType, content := partition.SplitMessage(message)
			switch msgType {
			case partition.CCommit:
				// commit消息
				//n.nl.Plog.Println("（主节点）收到commit消息")
				cmsg := new(partition.Commit)
				err := json.Unmarshal(content, cmsg)
				if err != nil {
					log.Panic(err)
				}
				// 获取request消息
				if r, ok := n.requestPool[string(cmsg.Digest)]; !ok {
					// request池子里没有相应的block
					n.nl.Plog.Println("（主节点）没有找到对应的request消息")
				} else {
					// 解析request message
					txs, _ := partition.Decode(r.Msg)
					// 区块上链
					time.Sleep(time.Duration(params.On_blockchain) * time.Millisecond)
					n.nl.Plog.Println("区块在本地上链,共识轮次为", n.sequenceID)
					//seqNow := n.sequenceID
					n.sequenceID += 1

					// 判断共识完的交易类型
					txExcuted := make([]*partition.SimpleTX, 0)

					// 生成relay pool 并且收集已经执行完的交易
					n.Txpool.RelayPool = make(map[uint64][]*partition.SimpleTX)
					relay1Txs := make([]*partition.SimpleTX, 0)

					for _, tx := range txs {
						rsid := n.Get_PartitionMap(tx.Recipient)
						if rsid != n.sid {
							ntx := tx
							ntx.Relayed = true
							n.Txpool.AddRelayTx(ntx, rsid)
							relay1Txs = append(relay1Txs, tx)
						} else {
							txExcuted = append(txExcuted, tx)
						}
					}
					// 发送relay txs
					for sid := uint64(0); sid < uint64(params.Partition_shardnum); sid++ {
						if sid == n.sid {
							continue
						}
						relay := partition.Relay{
							Txs:           n.Txpool.RelayPool[sid],
							SenderShardID: n.sid,
							SenderSeq:     n.sequenceID,
						}
						rByte, err := json.Marshal(relay)
						if err != nil {
							log.Panic()
						}
						msg_send := partition.MergeMessage(partition.CRelay, rByte)
						n.nl.Plog.Printf("send relay txs to %s, tx length %d \n", params.Ip_nodetable[sid], len(n.Txpool.RelayPool[sid]))
						go networks.TcpDial(msg_send, params.Ip_nodetable[sid])
						n.nl.Plog.Printf("S%d : sended relay txs to %d\n", n.sid, sid)
					}
					n.Txpool.ClearRelayPool()

					// 构建blockinfo
					bim := partition.BlockInfoMsg{
						BlockBodyLength: len(txs), //处理交易的数量
						ExcutedTxs:      txExcuted,

						ProposeTime:   r.ReqTime,
						CommitTime:    time.Now(),
						SenderShardID: n.sid,

						Relay1Txs:   relay1Txs,
						Relay1TxNum: uint64(len(relay1Txs)),

						//ArriveRate: n.Txpool.CalculateArrivalRate(),
						Queuelen: len(n.Txpool.TxQueue),
					}
					bByte, err := json.Marshal(bim)
					if err != nil {
						log.Panic()
					}
					// 给supervisor发送区块信息
					msg_send := partition.MergeMessage(partition.CBlockInfo, bByte)
					n.nl.Plog.Println("（主节点）给supervisor发送区块信息,区块长度为", bim.BlockBodyLength)
					networks.TcpDial(msg_send, ip_supervisor)
					//打印本次共识后，一些信息
					n.Txpool.GetLocked()
					n.writeCSVline([]string{strconv.Itoa(len(n.Txpool.TxQueue)), strconv.FormatFloat(n.Txpool.CalculateArrivalRate(), 'f', 3, 64)})
					n.Txpool.GetUnlocked()
				}
			}

		default:
			// indbox中没有消息，继续等待
			//time.Sleep(time.Duration(1) * time.Second)
		}
	}
}

func (n *Node) Get_PartitionMap(key string) uint64 {
	n.pmlock.RLock()
	defer n.pmlock.RUnlock()
	if _, ok := n.PartitionMap[key]; !ok {
		n.PartitionMap[key] = uint64(utils.Addr2Shard(key))
	}

	return n.PartitionMap[key]
}

func (n *Node) ProcessMessages() {
	ln, err := net.Listen("tcp", params.Ip_nodetable[n.sid])
	fmt.Printf("node begins listening：%s\n", params.Ip_nodetable[n.sid])
	if err != nil {
		log.Panic(err)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go n.handleClientRequest(conn)
	}
}

func (n *Node) handleClientRequest(con net.Conn) {
	defer con.Close()
	clientReader := bufio.NewReader(con)
	for {
		clientRequest, err := clientReader.ReadBytes('\n')
		switch err {
		case nil:
			params.TcpLock.Lock()
			n.handleMessage(clientRequest)
			params.TcpLock.Unlock()
		case io.EOF:
			log.Println("client closed the connection by terminating the process")
			return
		default:
			log.Printf("error: %v\n", err)
			return
		}
	}
}

func (n *Node) handleMessage(msg []byte) {
	msgType, content := partition.SplitMessage(msg)
	switch msgType {
	case partition.CRelay:
		n.handleRelay(content)
	case partition.CInject:
		n.handleInjectTx(content)
	}
}

func (n *Node) handleRelay(content []byte) {
	relay := new(partition.Relay)
	err := json.Unmarshal(content, relay)
	if err != nil {
		log.Panic(err)
	}
	n.nl.Plog.Printf("S%d : has received relay txs from shard %d, the senderSeq is %d\n", n.sid, relay.SenderShardID, relay.SenderSeq)
	n.Txpool.AddTxs2Pool(relay.Txs)
}

func (n *Node) handleInjectTx(content []byte) {
	it := new(partition.InjectTxs)
	err := json.Unmarshal(content, it)
	if err != nil {
		log.Panic(err)
	}
	//n.nl.Plog.Println("s", n.sid, "收到supervisor的inject消息", it.Txs)
	n.nl.Plog.Printf("S%d : has received inject txs, txs: %d\n", n.sid, len(it.Txs))
	n.Txpool.AddTxs2Pool(it.Txs)
}

func (n *Node) writeCSVline(str []string) {
	dirpath := params.DataWrite_path + "txpool"
	err := os.MkdirAll(dirpath, os.ModePerm)
	if err != nil {
		log.Panic(err)
	}

	targetPath := dirpath + "/Shard" + strconv.Itoa(int(n.sid)) + ".csv"
	f, err := os.Open(targetPath)
	if err != nil && os.IsNotExist(err) {
		file, er := os.Create(targetPath)
		if er != nil {
			panic(er)
		}
		defer file.Close()

		w := csv.NewWriter(file)
		//title := []string{"seq", "txpool size", "excuted tx", "tx1", "tx2"}
		title := []string{"txpool size", "arrive rate"}
		w.Write(title)
		w.Flush()
		w.Write(str)
		w.Flush()
	} else {
		file, err := os.OpenFile(targetPath, os.O_APPEND|os.O_RDWR, 0666)

		if err != nil {
			log.Panic(err)
		}
		defer file.Close()
		writer := csv.NewWriter(file)
		err = writer.Write(str)
		if err != nil {
			log.Panic()
		}
		writer.Flush()
	}

	f.Close()
}
