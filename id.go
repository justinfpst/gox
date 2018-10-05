package types

import (
	"errors"
	"log"
	"strings"
	"time"
)

// Change to int64, as https://github.com/golang/go/issues/12401 is fixed in golang v1.6
type ID int64

// JSON中数字表示为double，double整数部分最大值为2^53，由于部分JSON库默认不支持int64，因此控制在53bit内比较好
// id由time+shard+seq组成
// 若业务多可扩充shard，并发高可扩充seq. 由于time在最高位,故扩展后的id集合与原id集合不会出现交集,可保持全局唯一

const DefaultShardBitSize = 8 // 最多128个shard
const DefaultSeqBitSize = 8   // 每个shard每ms不能超过128次调用

var epoch time.Time
var DefaultSnakeIDGenerator IDGenerator

func init() {
	epoch = time.Date(2018, time.January, 2, 15, 4, 5, 0, time.UTC)
	DefaultSnakeIDGenerator = NewSnakeIDGenerator(DefaultShardBitSize, DefaultSeqBitSize, NextMilliseconds, GetShardIDByIP, DefaultCounter)
}

// NewID returns new ID created by default id generator
func NextID() ID {
	return DefaultSnakeIDGenerator.NextID()
}

// ShortString returns a short representation of id
func (i ID) ShortString() string {
	if i < 0 {
		panic("invalid id")
	}
	var bytes [16]byte
	k := int64(i)
	n := 15
	for {
		j := k % 62
		switch {
		case j <= 9:
			bytes[n] = byte('0' + j)
		case j <= 35:
			bytes[n] = byte('A' + j - 10)
		default:
			bytes[n] = byte('a' + j - 36)
		}
		k /= 62
		if k == 0 {
			return string(bytes[n:])
		}
		n--
	}
}

func ParseShortID(s string) (ID, error) {
	if len(s) == 0 {
		return 0, errors.New("parse error")
	}

	var bytes = []byte(s)
	var k int64
	var v int64
	for _, b := range bytes {
		switch {
		case b >= '0' && b <= '9':
			v = int64(b - '0')
		case b >= 'A' && b <= 'Z':
			v = int64(10 + b - 'A')
		case b >= 'a' && b <= 'z':
			v = int64(36 + b - 'a')
		default:
			return 0, errors.New("parse error")
		}
		k = k*62 + v
	}
	return ID(k), nil
}

const prettyTableSize = 34

var prettyTable = [prettyTableSize]byte{
	'1', '2', '3', '4', '5', '6', '7', '8', '9',
	'A', 'B', 'C', 'D', 'E', 'F', 'G',
	'H', 'I', 'J', 'K', 'L', 'M', 'N',
	'P', 'Q',
	'R', 'S', 'T',
	'U', 'V', 'W',
	'X', 'Y', 'Z'}

// PrettyString returns a incasesensitive pretty representation of id
func (i ID) PrettyString() string {
	if i < 0 {
		panic("invalid id")
	}
	var bytes [16]byte
	k := int64(i)
	n := 15

	for {
		bytes[n] = prettyTable[k%prettyTableSize]
		k /= prettyTableSize
		if k == 0 {
			return string(bytes[n:])
		}
		n--
	}
}

func ParsePrettyID(s string) (ID, error) {
	if len(s) == 0 {
		return 0, errors.New("parse error")
	}

	s = strings.ToUpper(s)
	var bytes = []byte(s)
	var k int64
	for _, b := range bytes {
		i := searchPrettyTable(b)
		if i <= 0 {
			return 0, errors.New("parse error")
		}
		k = k*prettyTableSize + int64(i)
	}
	return ID(k), nil
}

func searchPrettyTable(v byte) int {
	left := 0
	right := prettyTableSize - 1
	for right >= left {
		mid := (left + right) / 2
		if prettyTable[mid] == v {
			return mid
		} else if prettyTable[mid] > v {
			right = mid - 1
		} else {
			left = mid + 1
		}
	}

	return -1
}

type SnakeIDGenerator struct {
	seqBitSize   uint
	shardBitSize uint

	timestampGetter NumberGetter
	shardIDGetter   NumberGetter
	seqNumGetter    NumberGetter
}

func NewSnakeIDGenerator(shardBitSize, seqBitSize uint, timestampGetter, shardIDGetter, seqNumGetter NumberGetter) *SnakeIDGenerator {
	if seqBitSize < 1 || seqBitSize > 16 {
		panic("seqBitSize should be [1,16]")
	}

	if shardBitSize < 0 || shardBitSize > 8 {
		panic("shardBitSize should be [0,8]")
	}

	if shardBitSize+seqBitSize >= 20 {
		panic("shardBitSize + seqBitSize should be less than 20")
	}

	return &SnakeIDGenerator{
		seqBitSize,
		shardBitSize,
		timestampGetter,
		shardIDGetter,
		seqNumGetter,
	}
}

func (g *SnakeIDGenerator) Clone() *SnakeIDGenerator {
	return &*g
}

func (g *SnakeIDGenerator) NextID() ID {
	id := ID(g.seqNumGetter.GetNumber() % (1 << g.seqBitSize))
	id |= ID(KeepRightBits(g.shardIDGetter.GetNumber(), g.shardBitSize) << g.seqBitSize)
	id |= ID(g.timestampGetter.GetNumber() << (g.seqBitSize + g.shardBitSize))
	return ID(id)
}

type IDGenerator interface {
	NextID() ID
}

type NumberGetter interface {
	GetNumber() int64
}

type NumberGetterFunc func() int64

func (f NumberGetterFunc) GetNumber() int64 {
	return f()
}

var NextSecond NumberGetterFunc = func() int64 {
	return time.Since(epoch).Nanoseconds() / 1e9
}

var NextMilliseconds NumberGetterFunc = func() int64 {
	return time.Since(epoch).Nanoseconds() / 1e6
}

var GetShardIDByIP NumberGetterFunc = func() int64 {
	ip, err := GetOutboundIP()
	if err != nil {
		log.Fatal(err)
	}

	ipBytes := []byte(ip)
	var num int64 = 0
	for i := 0; i < 8 && i < len(ipBytes); i++ {
		num <<= 8
		num |= int64(ipBytes[i])
	}
	return num
}

func KeepRightBits(i int64, bitSize uint) int64 {
	return ((i >> bitSize) << bitSize) ^ i
}
