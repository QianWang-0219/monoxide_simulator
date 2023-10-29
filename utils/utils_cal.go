package utils

import (
	"log"
	"math"
	"math/rand"
	"simple_go/monoxide/params"
	"strconv"
	"time"
)

// the default method 将 由合法的十六进制数字组成的字符串 作为参数传递给Addr2Shard函数
func Addr2Shard(addr string) int {
	last16_addr := addr[len(addr)-8:]
	num, err := strconv.ParseUint(last16_addr, 16, 64)
	if err != nil {
		log.Panic(err)
	}
	return int(num) % params.Partition_shardnum
}

func Intersection(a, b []uint64) []uint64 {
	set := make(map[uint64]bool)
	for _, v := range a {
		set[v] = true
	}
	var res []uint64
	for _, v := range b {
		if set[v] {
			res = append(res, v)
			set[v] = false
		}
	}
	return res
}

func CheckChannel(ch <-chan []byte) bool {
	select {
	case <-ch:
		// 如果通道中有消息，则返回 true
		return true
	default:
		// 如果通道中没有消息，则返回 false
		return false
	}
}

func FindMinMax(nums []int) (int, int) {
	min := math.MaxInt64
	max := math.MinInt64

	for _, num := range nums {
		if num < min {
			min = num
		}
		if num > max {
			max = num
		}
	}

	return min, max
}

// func FindMaxIndexINf(nums []float64) (int, float64) {
// 	max := math.Inf(-1)
// 	maxIndex := -1

// 	for i, num := range nums {
// 		if num > max {
// 			max = num
// 			maxIndex = i
// 		}
// 	}

// 	return maxIndex, max
// }

func FindMaxIndexINf(nums []float64) (int, float64) {
	max := math.Inf(-1)
	maxIndex := -1
	equalMaxIndices := []int{}

	for i, num := range nums {
		if num > max {
			max = num
			maxIndex = i
			equalMaxIndices = []int{i} // 如果有更大的数出现，重置相等最大值的索引
		} else if num == max {
			equalMaxIndices = append(equalMaxIndices, i) // 将相等最大值的索引添加到列表中
		}
	}

	// 如果有相等的最大值索引，从列表中随机选择一个索引
	if len(equalMaxIndices) > 1 {
		rand.Seed(time.Now().UnixNano())
		randomIndex := equalMaxIndices[rand.Intn(len(equalMaxIndices))]
		maxIndex = randomIndex
	}

	return maxIndex, max
}
