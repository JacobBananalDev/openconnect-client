package main

import (
	"testing"

	"github.com/JacobBananalDev/openconnect-client/internal/ipc"
)

func TestAuthorizePeer(t *testing.T) {
	const gid = 1234
	tests := []struct {
		name string
		peer ipc.PeerCredentials
		want bool
	}{
		{"root", ipc.PeerCredentials{UID: 0}, true},
		{"primary group", ipc.PeerCredentials{UID: 99, GID: gid}, true},
		{"supplementary group", ipc.PeerCredentials{UID: 99, GID: 22, Groups: []uint32{12, gid}}, true},
		{"unauthorized", ipc.PeerCredentials{UID: 99, GID: 22, Groups: []uint32{12}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := authorizePeer(tt.peer, gid) == nil; got != tt.want {
				t.Errorf("authorized = %v, want %v", got, tt.want)
			}
		})
	}
}
