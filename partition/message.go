package partition

import (
	"crypto/sha256"
	"encoding/json"
	"log"
	"time"
)

var prefixMSGtypeLen = 30

const (
	CInject    string = "inject"
	CRequest   string = "request"
	CCommit    string = "commit"
	CBlockInfo string = "BlockInfo"
	CRelay     string = "relay"
)

type InjectTxs struct {
	Txs       []*SimpleTX
	ToShardID uint64
}

type Request struct {
	Msg     []byte    // request message
	ReqTime time.Time // request time
}

type Commit struct {
	Digest []byte // To identify which request is prepared by this node
	SeqID  uint64
}

type BlockInfoMsg struct {
	BlockBodyLength int
	ExcutedTxs      []*SimpleTX // txs which are excuted completely

	ProposeTime   time.Time // record the propose time of this block (txs)
	CommitTime    time.Time // record the commit time of this block (txs)
	SenderShardID uint64

	// for transaction relay
	Relay1TxNum uint64      // the number of cross shard txs
	Relay1Txs   []*SimpleTX // cross transactions in chain first time

	ArriveRate float64
	Queuelen   int
}

type Relay struct {
	Txs           []*SimpleTX
	SenderShardID uint64
	SenderSeq     uint64
}

func MergeMessage(msgType string, content []byte) []byte {
	b := make([]byte, prefixMSGtypeLen)
	for i, v := range []byte(msgType) {
		b[i] = v
	}
	merge := append(b, content...)
	return merge
}

func SplitMessage(message []byte) (string, []byte) {
	msgTypeBytes := message[:prefixMSGtypeLen]
	msgType_pruned := make([]byte, 0)
	for _, v := range msgTypeBytes {
		if v != byte(0) {
			msgType_pruned = append(msgType_pruned, v)
		}
	}
	msgType := string(msgType_pruned)
	content := message[prefixMSGtypeLen:]
	return msgType, content
}

// get the digest of request
func GetDigest(r *Request) []byte {
	b, err := json.Marshal(r)
	if err != nil {
		log.Panic(err)
	}
	hash := sha256.Sum256(b)
	return hash[:]
}
