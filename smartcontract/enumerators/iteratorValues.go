package enumerators

import (
	"github.com/elastos/Elastos.ELA.SideChain/database"
)

type IteratorValues struct {
	iter database.Iterator
}

func NewIteratorValues(iter database.Iterator) *IteratorValues {
	var iterValues IteratorValues
	iterValues.iter = iter
	return &iterValues
}

func (iter *IteratorValues) Next() bool {
	return iter.iter.Next()
}

func (iter *IteratorValues) Value() []byte {
	return iter.iter.Value()
}

func (iter *IteratorValues) Dispose()  {
	iter.iter.Release()
}

func (iter *IteratorValues) Bytes() []byte {
	return iter.iter.Value()
}