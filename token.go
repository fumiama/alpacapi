package alpacapi

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strconv"
	"time"
	"unsafe"

	"github.com/wdvxdr1123/ZeroBot/extension/rate"
)

// Token 请求口令
type Token struct {
	md5 [md5.Size]byte // md5 以下数据的md5
	cnt uint8          // cnt 请求次数上限
	per uint8          // per 每几分钟
	unm [6]byte        // unm 用户名, 必须6字节
	out [8]byte        // out 过期时间, unix 时间戳
	sed [8]byte        // sed 随机数
}

// NewToken per = 0 for not limit. cnt = 0 is invalid
func NewToken(cnt, per uint8, unm string, out, sed int64) (t Token) {
	t.cnt = cnt
	t.per = per
	copy(t.unm[:], unm[:6]) // panic if len(unm) < 6
	binary.LittleEndian.PutUint64(t.out[:], uint64(out))
	if sed == 0 {
		_, _ = rand.Read(t.sed[:])
	} else {
		binary.LittleEndian.PutUint64(t.sed[:], uint64(sed))
	}
	raw := (*[unsafe.Sizeof(Token{}) - md5.Size]byte)(unsafe.Add(unsafe.Pointer(&t), md5.Size))
	t.md5 = md5.Sum(raw[:])
	return
}

// ParseToken parse from raw data
func ParseToken(b *[unsafe.Sizeof(Token{})]byte) *Token {
	return (*Token)(unsafe.Pointer(b))
}

// ParseToken parse from hex string
func ParseTokenString(s string) (*Token, error) {
	if len(s) != int(unsafe.Sizeof(Token{})*2) {
		return nil, errors.New("len(s) must be " + strconv.Itoa(int(unsafe.Sizeof(Token{})*2)))
	}
	data, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return (*Token)(*(*unsafe.Pointer)(unsafe.Pointer(&data))), nil
}

// IsValid check md5, cnt != 0 and not out of expire date
func (t *Token) IsValid() bool {
	raw := (*[unsafe.Sizeof(Token{}) - md5.Size]byte)(unsafe.Add(unsafe.Pointer(t), md5.Size))
	return t.md5 == md5.Sum(raw[:]) && t.cnt != 0 &&
		time.Now().Unix() < int64(binary.LittleEndian.Uint64(t.out[:]))
}

// NewLimiter ...
func (t *Token) NewLimiter() *rate.Limiter {
	if t.per == 0 || t.cnt == 0 {
		return nil
	}
	return rate.NewLimiter(time.Minute*time.Duration(t.per), int(t.cnt))
}

// String prints unm
func (t *Token) String() string {
	return string(t.unm[:])
}

// Hex of token
func (t *Token) Hex() string {
	raw := (*[unsafe.Sizeof(Token{})]byte)(unsafe.Pointer(t))
	return hex.EncodeToString(raw[:])
}
