package partition

import (
	"simple_go/monoxide/params"
	"time"
)

type MeasureRes struct {
	// for tps
	excutedTxNum float64 //
	relayTx      map[string]bool
	startTime    time.Time //第一笔交易开始共识的时间 （打包区块之后，主节点生成request的时间）
	endTime      time.Time //最后一笔交易reply的时间

	// for latency
	latency float64 //每笔完成的交易延迟之和（区块reply时间-tx加入交易池的时间）
	txNum   float64

	// for ctx
	totTxNum  float64
	totCtxNum float64
	//b1num, b2num int
	relayTxRecord map[string]bool

	// for avg arrive rate
	arriveRateTable []float64

	queueTable []int

	// for avg service rate

}

func NewMeasureRes() *MeasureRes {
	return &MeasureRes{
		excutedTxNum: float64(0),
		relayTx:      make(map[string]bool),

		latency: float64(0),
		txNum:   float64(0),

		totTxNum:      float64(0),
		totCtxNum:     float64(0),
		relayTxRecord: make(map[string]bool),
		// b1num:           0,
		// b2num:           0,
		arriveRateTable: make([]float64, params.Partition_shardnum),
		queueTable:      make([]int, params.Partition_shardnum),
	}
}

func (mr *MeasureRes) UpdateMeasureRecord(b *BlockInfoMsg) {
	if b.BlockBodyLength == 0 {
		return
	}

	// update tps
	earliestTime := b.ProposeTime
	latestTime := b.CommitTime

	if mr.startTime.IsZero() || mr.startTime.After(earliestTime) {
		mr.startTime = earliestTime
	}
	if mr.endTime.IsZero() || latestTime.After(mr.endTime) {
		mr.endTime = latestTime
	}

	for _, tx := range b.Relay1Txs {
		mr.relayTx[string(tx.TxHash)] = true
	}
	mr.excutedTxNum += float64(b.Relay1TxNum) / 2
	for _, tx := range b.ExcutedTxs {
		if _, ok := mr.relayTx[string(tx.TxHash)]; ok {
			mr.excutedTxNum += 0.5
		} else {
			mr.excutedTxNum += 1
		}
	}

	// update latency
	mTime := b.CommitTime
	txs := b.ExcutedTxs

	for _, tx := range txs {
		if !tx.Time.IsZero() {
			mr.latency += mTime.Sub(tx.Time).Seconds()
			//fmt.Println("+latency",mTime,tx.Time)
			mr.txNum++
		}
	}

	// update ctx
	for _, r1tx := range b.Relay1Txs {
		mr.relayTxRecord[string(r1tx.TxHash)] = true
		mr.totCtxNum += 0.5
		mr.totTxNum += 0.5
	}
	for _, tx := range b.ExcutedTxs {
		if _, ok := mr.relayTxRecord[string(tx.TxHash)]; !ok {
			mr.totTxNum += 1
		} else {
			mr.totCtxNum += 0.5
			mr.totTxNum += 0.5
		}
	}

	// update arrive rate
	mr.arriveRateTable[b.SenderShardID] = b.ArriveRate

	// updata queue len
	mr.queueTable[b.SenderShardID] = b.Queuelen
}

func (mr *MeasureRes) OutputRecord() (tps float64, latency float64, ctxrate float64, queueTable []int) {
	// tps
	timeGap := mr.endTime.Sub(mr.startTime).Seconds()
	//fmt.Println("timeGap", mr.endTime, mr.startTime)
	tps = mr.excutedTxNum / timeGap

	// latency
	latency = mr.latency / mr.txNum

	// ctx
	ctxrate = mr.totCtxNum / mr.totTxNum

	// arrive rate
	//arriveRateTable = mr.arriveRateTable

	queueTable = mr.queueTable
	return
}
