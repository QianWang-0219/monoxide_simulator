package partition

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"log"
	"sync"
	"time"
)

type SimpleTX struct {
	Recipient string
	Sender    string
	Nonce     uint64

	TxHash []byte
	Time   time.Time // the time adding in pool

	Relayed bool
}

func NewSimpleTX(sender, recipient string, nonce uint64) *SimpleTX {
	tx := &SimpleTX{
		Recipient: recipient,
		Sender:    sender,
		Nonce:     nonce,
	}
	hash := sha256.Sum256(tx.Encode())
	tx.TxHash = hash[:]
	tx.Relayed = false
	return tx
}

func (tx *SimpleTX) Encode() []byte {
	var buff bytes.Buffer

	enc := gob.NewEncoder(&buff)
	err := enc.Encode(tx)
	if err != nil {
		log.Panic(err)
	}

	return buff.Bytes()
}

func Data2tx(data []string, nonce uint64) (*SimpleTX, bool) {
	tx := NewSimpleTX(data[0][:], data[1][:], nonce)
	return tx, true
}

type TxPool struct {
	TxQueue        []*SimpleTX            // transaction Queue
	RelayPool      map[uint64][]*SimpleTX //designed for sharded blockchain, from Monoxide
	lock           sync.Mutex
	ArrivalHistory []time.Time // 记录交易到达时间的历史记录
}

func NewTxPool() *TxPool {
	return &TxPool{
		TxQueue:   make([]*SimpleTX, 0),
		RelayPool: make(map[uint64][]*SimpleTX),
	}
}

// Pack transactions for a proposal
func (txpool *TxPool) PackTxs(max_txs uint64) []*SimpleTX {
	txpool.lock.Lock()
	defer txpool.lock.Unlock()

	txNum := max_txs
	if uint64(len(txpool.TxQueue)) < txNum {
		txNum = uint64(len(txpool.TxQueue))
	}
	txs_Packed := txpool.TxQueue[:txNum]    //从交易队列中打包前txNum条交易
	txpool.TxQueue = txpool.TxQueue[txNum:] //更新交易队列

	return txs_Packed
}

// Add a list of transactions to the pool
func (txpool *TxPool) AddTxs2Pool(txs []*SimpleTX) {
	txpool.lock.Lock()
	defer txpool.lock.Unlock()
	for _, tx := range txs {
		if tx.Time.IsZero() {
			tx.Time = time.Now()
		}
		txpool.TxQueue = append(txpool.TxQueue, tx)
		txpool.ArrivalHistory = append(txpool.ArrivalHistory, time.Now())
	}
}

// txpool get locked
func (txpool *TxPool) GetLocked() {
	txpool.lock.Lock()
}

// txpool get unlocked
func (txpool *TxPool) GetUnlocked() {
	txpool.lock.Unlock()
}

// Relay transactions
func (txpool *TxPool) AddRelayTx(tx *SimpleTX, sid uint64) {
	txpool.lock.Lock()
	defer txpool.lock.Unlock()
	_, ok := txpool.RelayPool[sid]
	if !ok {
		txpool.RelayPool[sid] = make([]*SimpleTX, 0)
	}
	txpool.RelayPool[sid] = append(txpool.RelayPool[sid], tx)
}

// get the length of ClearRelayPool
func (txpool *TxPool) ClearRelayPool() {
	txpool.lock.Lock()
	defer txpool.lock.Unlock()
	txpool.RelayPool = nil
}

func Encode(txs []*SimpleTX) []byte {
	var buff bytes.Buffer
	enc := gob.NewEncoder(&buff)
	err := enc.Encode(txs)
	if err != nil {
		log.Panic(err)
	}
	return buff.Bytes()
}

func Decode(b []byte) ([]*SimpleTX, error) {
	var txs []*SimpleTX
	dec := gob.NewDecoder(bytes.NewReader(b))
	err := dec.Decode(&txs)
	if err != nil {
		return nil, err
	}
	return txs, nil
}

func (txpool *TxPool) CalculateArrivalRate() float64 {
	now := time.Now()
	count := 0
	for i := len(txpool.ArrivalHistory) - 1; i >= 0; i-- {
		arrivalTime := txpool.ArrivalHistory[i]
		if now.Sub(arrivalTime) <= 3*time.Second {
			count++
		} else {
			break
		}
	}
	arrivalRate := float64(count) / float64(3*time.Second.Seconds()) // 到达率 = 到达的交易数量 / 时间间隔
	return arrivalRate
}