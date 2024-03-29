package gox

import (
	"fmt"
	"math/rand"
	osuser "os/user"
	"strings"
	"time"
)

func randomString() string {
	b := &strings.Builder{}
	addrs, err := GetMacAddrs()
	if err == nil {
		for _, a := range addrs {
			b.WriteString(a)
		}
	}

	u, err := osuser.Current()
	if err == nil {
		b.WriteString(u.Name)
		b.WriteString(u.Username)
		b.WriteString(u.Gid)
		b.WriteString(u.HomeDir)
		b.WriteString(u.Uid)
	}

	b.WriteString(GetIP().String())
	b.WriteString(time.Now().String())
	b.WriteString(NextID().ShortString())
	b.WriteString(fmt.Sprint(rand.Int()))
	return b.String()
}

func UniqueID() string {
	return SHA1(randomString())
}

func UniqueID32() string {
	return MD5(randomString())
}

func UniqueID40() string {
	return SHA1(randomString())
}

func UniqueID64() string {
	return SHA256(randomString())
}
