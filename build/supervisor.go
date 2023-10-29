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
	"simple_go/monoxide/params"
	"simple_go/monoxide/partition"
	"simple_go/monoxide/slog"
	"simple_go/monoxide/utils"
	"sync"
	"time"
)

type Supervisor struct {
	partitionLock sync.Mutex
	ModifiedMap   map[string]uint64

	sl     *slog.SupervisorLog
	itxnum uint64
	ctxnum uint64

	measureRes      *partition.MeasureRes
	arriveRateTable []float64
	queueTable      []int
}

func NewSupervisor() *Supervisor {
	sl := slog.NewSupervisorLog()

	return &Supervisor{
		ModifiedMap:     make(map[string]uint64),
		itxnum:          0,
		ctxnum:          0,
		sl:              sl,
		measureRes:      partition.NewMeasureRes(),
		arriveRateTable: make([]float64, params.Partition_shardnum),
		queueTable:      make([]int, params.Partition_shardnum),
	}
}

func (s *Supervisor) SupervisorTxHandling() {
	txlist := make([]*partition.SimpleTX, 0)

	//读交易
	var nowDataNum int
	txfile, err := os.Open("/Volumes/T7/paper/实验/ethGraph/graphDataTOTAL2.csv")
	if err != nil {
		log.Panic(err)
	}
	defer txfile.Close()
	reader := csv.NewReader(txfile)

	for {
		data, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Panic(err)
		}
		if tx, ok := partition.Data2tx(data, uint64(nowDataNum)); ok {
			txlist = append(txlist, tx)
			nowDataNum++
		} else {
			continue
		}

		// batch sending condition   batchsize;totaldatanum
		if len(txlist) == BatchSize || nowDataNum == TotalDataSize {

			fmt.Println("now data num =", nowDataNum)
			s.txsending(txlist)

			txlist = make([]*partition.SimpleTX, 0)
		}

		if nowDataNum == TotalDataSize {
			break
		}
	}
}

func (s *Supervisor) txsending(txlist []*partition.SimpleTX) {
	sendToShard := make(map[uint64][]*partition.SimpleTX)

	for idx := 0; idx <= len(txlist); idx++ {
		if idx > 0 && (idx%InjectSpeed == 0 || idx == len(txlist)) {
			for sid := uint64(0); sid < uint64(params.Partition_shardnum); sid++ {
				if len(sendToShard[sid]) == 0 {
					continue
				}
				it := partition.InjectTxs{
					Txs:       sendToShard[sid],
					ToShardID: sid,
				}

				itByte, err := json.Marshal(it)
				if err != nil {
					log.Panic(err)
				}
				send_msg := partition.MergeMessage(partition.CInject, itByte)

				networks.TcpDial(send_msg, params.Ip_nodetable[sid])
				s.sl.Slog.Printf("send %d txs to %d\n", len(sendToShard[sid]), sid)
			}
			sendToShard = make(map[uint64][]*partition.SimpleTX)
			time.Sleep(time.Second) // 每隔1s发送InjectSpeed笔交易
		}
		if idx == len(txlist) {
			break
		}
		tx := txlist[idx]

		s.partitionLock.Lock()
		senderid := s.fetchModifiedMap(tx.Sender)
		s.partitionLock.Unlock()
		sendToShard[senderid] = append(sendToShard[senderid], tx)
	}
}

// 查找账户的分片地址，如果是新账户需要分配一个分片id
func (s *Supervisor) fetchModifiedMap(key string) uint64 {
	if _, ok := s.ModifiedMap[key]; !ok {
		//s.sl.Slog.Printf("fetchModifiedMap: %s not found\n", key)
		s.ModifiedMap[key] = uint64(utils.Addr2Shard(key))
	}
	return s.ModifiedMap[key]
}

func (s *Supervisor) superListen() {
	s.sl.Slog.Printf("superListen")
	ln, err := net.Listen("tcp", ip_supervisor)
	if err != nil {
		log.Panic(err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go s.handleClientRequest(conn)
	}

}

func (s *Supervisor) handleClientRequest(con net.Conn) {
	defer con.Close()
	clientReader := bufio.NewReader(con)
	for {
		clientRequest, err := clientReader.ReadBytes('\n')
		switch err {
		case nil:
			params.TcpLock.Lock()
			s.handleMessage(clientRequest)
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

func (s *Supervisor) handleMessage(msg []byte) {
	msgType, content := partition.SplitMessage(msg)
	switch msgType {
	case partition.CBlockInfo:
		s.handleBlockInfos(content)
	default:
	}
}

func (s *Supervisor) handleBlockInfos(content []byte) {
	bim := new(partition.BlockInfoMsg)
	err := json.Unmarshal(content, bim)
	if err != nil {
		log.Panic()
	}

	// 解析共识完的交易
	// s.sl.Slog.Printf("received from S %d, tx length=%d, excuted tx length = %d\n", bim.SenderShardID, bim.BlockBodyLength, len(bim.ExcutedTxs))

	// s.sl.Slog.Println("开始共识的时间", bim.ProposeTime, "reply的时间", bim.CommitTime)
	// for _, tx := range bim.Relay1Txs {
	// 	s.sl.Slog.Println("relay", tx)
	// 	s.sl.Slog.Println("加入交易池的时间", tx.Time)
	// }
	// for _, tx := range bim.ExcutedTxs {
	// 	s.sl.Slog.Println("excuted", tx.Sender, tx.Recipient)
	// 	s.sl.Slog.Println("加入交易池的时间", tx.Time)
	// }

	// 测量数据更新
	s.measureRes.UpdateMeasureRecord(bim)
	tps, latency, ctxrate, queueTable := s.measureRes.OutputRecord()
	//s.arriveRateTable = arriveRateTable
	s.queueTable = queueTable
	s.sl.Slog.Println("measureRes:", tps, latency, ctxrate, queueTable)

}
