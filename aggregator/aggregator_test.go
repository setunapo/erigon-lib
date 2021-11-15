/*
   Copyright 2021 Erigon contributors

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package aggregator

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/memdb"
)

func int160(i uint64) []byte {
	b := make([]byte, 20)
	binary.BigEndian.PutUint64(b[12:], i)
	return b
}

func int256(i uint64) []byte {
	b := make([]byte, 32)
	binary.BigEndian.PutUint64(b[24:], i)
	return b
}

func TestSimpleAggregator(t *testing.T) {
	tmpDir := t.TempDir()
	db := memdb.New()
	defer db.Close()
	a, err := NewAggregator(tmpDir, 16, 4)
	if err != nil {
		t.Fatal(err)
	}
	var rwTx kv.RwTx
	if rwTx, err = db.BeginRw(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer rwTx.Rollback()

	var w *Writer
	if w, err = a.MakeStateWriter(rwTx, 0); err != nil {
		t.Fatal(err)
	}
	var account1 = int256(1)
	if err = w.UpdateAccountData(int160(1), account1); err != nil {
		t.Fatal(err)
	}
	if err = w.Finish(); err != nil {
		t.Fatal(err)
	}
	if err = rwTx.Commit(); err != nil {
		t.Fatal(err)
	}
	var tx kv.Tx
	if tx, err = db.BeginRo(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	r := a.MakeStateReader(tx, 2)
	var acc []byte
	if acc, err = r.ReadAccountData(int160(1)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(acc, account1) {
		t.Errorf("read account %x, expected account %x", acc, account1)
	}
	a.Close()
}

func TestLoopAggregator(t *testing.T) {
	tmpDir := t.TempDir()
	db := memdb.New()
	defer db.Close()
	a, err := NewAggregator(tmpDir, 16, 4)
	if err != nil {
		t.Fatal(err)
	}
	var account1 = int256(1)
	var rwTx kv.RwTx
	defer func() {
		rwTx.Rollback()
	}()
	var tx kv.Tx
	defer func() {
		tx.Rollback()
	}()
	for blockNum := uint64(0); blockNum < 1000; blockNum++ {
		accountKey := int160(blockNum/10 + 1)
		//fmt.Printf("blockNum = %d\n", blockNum)
		if rwTx, err = db.BeginRw(context.Background()); err != nil {
			t.Fatal(err)
		}
		var w *Writer
		if w, err = a.MakeStateWriter(rwTx, blockNum); err != nil {
			t.Fatal(err)
		}
		if err = w.UpdateAccountData(accountKey, account1); err != nil {
			t.Fatal(err)
		}
		if err = w.Finish(); err != nil {
			t.Fatal(err)
		}
		if err = rwTx.Commit(); err != nil {
			t.Fatal(err)
		}
		if tx, err = db.BeginRo(context.Background()); err != nil {
			t.Fatal(err)
		}
		r := a.MakeStateReader(tx, blockNum+1)
		var acc []byte
		if acc, err = r.ReadAccountData(accountKey); err != nil {
			t.Fatal(err)
		}
		tx.Rollback()
		if !bytes.Equal(acc, account1) {
			t.Errorf("read account %x, expected account %x for block %d", acc, account1, blockNum)
		}
		account1 = int256(blockNum + 2)
	}
	if tx, err = db.BeginRo(context.Background()); err != nil {
		t.Fatal(err)
	}
	blockNum := uint64(1000)
	r := a.MakeStateReader(tx, blockNum)
	for i := uint64(0); i < blockNum/10+1; i++ {
		accountKey := int160(i)
		var expected []byte
		if i > 0 {
			expected = int256(i * 10)
		}
		var acc []byte
		if acc, err = r.ReadAccountData(accountKey); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(acc, expected) {
			t.Errorf("read account %x, expected account %x for block %d", acc, expected, i)
		}
	}
	tx.Rollback()
	a.Close()
}