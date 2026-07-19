package etcdleader

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const electionPrefix = "/bluetape4k/leader"

type electionPath struct {
	base string
	root string
	end  string
}

func electionPaths(opts leader.Options) electionPath {
	encode := base64.RawURLEncoding.EncodeToString
	base := fmt.Sprintf("%s/%s/%s", electionPrefix, encode([]byte(opts.KeyPrefix)), encode([]byte(opts.Group)))
	root := base + "/"
	return electionPath{
		base: base,
		root: root,
		end:  clientv3.GetPrefixRangeEnd(root),
	}
}

func requestedTTL(lease time.Duration) (int64, error) {
	if lease <= 0 {
		return 0, errors.New("etcd leader lease must be positive")
	}

	seconds := int64(lease / time.Second)
	if lease%time.Second != 0 {
		if seconds >= maxTTLSeconds() {
			return 0, errors.New("etcd leader lease exceeds duration range")
		}
		seconds++
	}
	if _, err := ttlDuration(seconds); err != nil {
		return 0, err
	}
	return seconds, nil
}

func ttlDuration(seconds int64) (time.Duration, error) {
	if seconds <= 0 {
		return 0, errors.New("etcd leader TTL must be positive")
	}
	if seconds > maxTTLSeconds() {
		return 0, errors.New("etcd leader TTL exceeds duration range")
	}
	return time.Duration(seconds) * time.Second, nil
}

func maxTTLSeconds() int64 {
	return int64(^uint64(0)>>1) / int64(time.Second)
}

func ownerToken(memberID string) (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("etcd leader owner token: %w", err)
	}
	return memberID + ":" + hex.EncodeToString(random[:]), nil
}
