package sharding

import (
	"encoding/binary"
	"hash/crc32"
)

func CustomHashCRC(biz_id int64, key string) uint64 {
	// 1. 序列化并加盐
	bizBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bizBytes, uint64(biz_id))
	salted := append(bizBytes, []byte(key)...)
	salted = append(salted, bizBytes...) // 二次加盐
	// 2. 计算CRC32并扩展为64位
	crc := crc32.ChecksumIEEE(salted)
	extendedHash := uint64(crc)<<32 | uint64(crc) // 防止高位丢失
	return extendedHash
}
