package repo

import (
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNetworkConfig(t *testing.T) {
	path := "./testdata"
	cfg, err := loadNetworkConfig(path, Genesis{Addresses: []string{"0xc7F999b83Af6DF9e67d0a37Ee7e900bF38b3D013",
		"0x79a1215469FaB6f9c63c1816b45183AD3624bE34",
		"0x97c8B516D19edBf575D72a172Af7F418BE498C37",
		"0xc0Ff2e0b3189132D815b8eb325bE17285AC898f8"}})

	require.Nil(t, err)
	peers, err := cfg.GetPeers()
	require.Nil(t, err)
	require.Equal(t, 4, len(peers))

	accounts := cfg.GetVpGenesisAccount()
	require.Equal(t, 4, len(accounts))

	vpAccounts := cfg.GetVpAccount()
	require.Equal(t, 4, len(vpAccounts))

	vpInfos := cfg.GetVpInfos()
	require.Equal(t, 4, len(vpInfos))
}

func TestRewriteNetworkConfig(t *testing.T) {
	infos := make([]*pb.VpInfo, 0)
	{
		infos = append(infos, &pb.VpInfo{
			Id:      1,
			Hosts:   []string{"/ip4/127.0.0.1/tcp/4001/p2p/"},
			Pid:     "QmQUcDYCtqbpn5Nhaw4FAGxQaSSNvdWfAFcpQT9SPiezbS",
			Account: "0xc7F999b83Af6DF9e67d0a37Ee7e900bF38b3D013",
		})
		infos = append(infos, &pb.VpInfo{
			Id:      2,
			Hosts:   []string{"/ip4/127.0.0.1/tcp/4002/p2p/"},
			Pid:     "QmQW3bFn8XX1t4W14Pmn37bPJUpUVBrBjnPuBZwPog3Qdy",
			Account: "0x79a1215469FaB6f9c63c1816b45183AD3624bE34",
		})
		infos = append(infos, &pb.VpInfo{
			Id:      3,
			Hosts:   []string{"/ip4/127.0.0.1/tcp/4003/p2p/"},
			Pid:     "QmXi58fp9ZczF3Z5iz1yXAez3Hy5NYo1R8STHWKEM9XnTL",
			Account: "0x97c8B516D19edBf575D72a172Af7F418BE498C37",
		})
		infos = append(infos, &pb.VpInfo{
			Id:      4,
			Hosts:   []string{"/ip4/127.0.0.1/tcp/4004/p2p/"},
			Pid:     "QmbmD1kzdsxRiawxu7bRrteDgW1ituXupR8GH6E2EUAHY4",
			Account: "0xc0Ff2e0b3189132D815b8eb325bE17285AC898f8",
		})
		infos = append(infos, &pb.VpInfo{
			Id:      5,
			Hosts:   []string{"/ip4/127.0.0.1/tcp/4005/p2p/"},
			Pid:     "QmbmD1kzdsxRiawxu7bRrteDgW1ituXupR8GH6E",
			Account: "0xc0Ff2e0b3189132D815b8eb325bE1728",
		})
	}
	err := RewriteNetworkConfig("./testdata", infos)
	require.Nil(t, err)
	err = RewriteNetworkConfig("./testdata", infos[:len(infos)-1])
	require.Nil(t, err)
}
