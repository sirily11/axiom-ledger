package ledger

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/axiomesh/axiom-kit/types"
)

func TestAccessTupleList_StorageKeys(t *testing.T) {
	tupleList := make(AccessTupleList, 10)

	tuple := &AccessTuple{
		StorageKeys: []types.Hash{},
	}

	tupleList = append(tupleList, *tuple)

	count := tupleList.StorageKeys()

	assert.Equal(t, count, 0)
}

func TestAccessList_Copy(t *testing.T) {
	acl := NewAccessList()
	addr := types.NewAddressByStr("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	hash0 := types.NewHashByStr("0xe9FC370DD36C9BD5f67cCfbc031C909F53A3d8bC7084C01362c55f2D42bA841c")
	hash1 := types.NewHashByStr("0xe9FC370DD36C9BD5f67cCfbc031C909F53A3d8bC7084C01362c55f2D42bA841d")
	hash2 := types.NewHashByStr("0xe9FC370DD36C9BD5f67cCfbc031C909F53A3d8bC7084C01362c55f2D42bA842d")

	addrExist, hashExist := acl.Contains(*addr, *hash2)
	assert.False(t, addrExist)
	assert.False(t, hashExist)

	acl.AddAddress(*addr)
	addrExist, hashExist = acl.Contains(*addr, *hash2)
	assert.True(t, addrExist)
	assert.False(t, hashExist)

	acl.AddSlot(*addr, *hash0)
	acl.AddSlot(*addr, *hash1)
	acl.AddSlot(*addr, *hash1)

	aclCopy := acl.Copy()

	assert.True(t, aclCopy.ContainsAddress(*addr))

	addrExist, hashExist = aclCopy.Contains(*addr, *hash2)
	assert.True(t, addrExist)
	assert.False(t, hashExist)
}
