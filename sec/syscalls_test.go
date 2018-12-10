package sec

import "testing"
import "github.com/stretchr/testify/assert"

func TestNameToInt(t *testing.T) {
	id, err := SCTbl.GetID("open")
	assert.Equal(t, id, 2, "id must be 2")
	assert.Nil(t, err, "err must be nil")
}

func TestIntToName(t *testing.T) {
	name, err := SCTbl.GetName(3)
	assert.Equal(t, name, "close", "name must be stat")
	assert.Nil(t, err, "err must be nil")
}

func TestNameToIntNotExisted(t *testing.T) {
	_, err := SCTbl.GetID("openn")
	assert.NotNil(t, err, "err must not be nil")
}

func TestIDNotExisted(t *testing.T) {
	_, err := SCTbl.GetName(-1)
	assert.NotNil(t, err, "err must not be nil")
}

func TestIDNotExisted2(t *testing.T) {
	_, err := SCTbl.GetName(1000)
	assert.NotNil(t, err, "err must not be nil")
}
