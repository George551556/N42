// Copyright 2023 The N42 Authors
// This file is part of the N42 library.
//
// The N42 library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The N42 library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the N42 library. If not, see <http://www.gnu.org/licenses/>.

package rawdb

import (
	"context"
	"github.com/c2h5oh/datasize"
	"github.com/ledgerwatch/erigon-lib/common/cmp"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/n42blockchain/N42/modules"
	"github.com/n42blockchain/N42/params"
	"golang.org/x/sync/semaphore"
	"runtime"

	log2 "github.com/ledgerwatch/log/v3"
)

//func TestPutReward(t *testing.T) {
//	db, err := OpenDatabase()
//	if err != nil {
//		t.Error(err)
//	}
//	tx, err := db.BeginRw(context.TODO())
//	if err != nil {
//		t.Error(err)
//	}
//	if err := tx.CreateBucket("Reward"); err != nil {
//		t.Error(err)
//	}
//	type args struct {
//		key string
//		val *RewardEntry
//	}
//	tests := []struct {
//		name    string
//		args    args
//		wantErr bool
//	}{{
//		name: "t1",
//		args: args{
//			key: "qwe123",
//			val: &RewardEntry{
//				Address:   []byte("123"),
//				Value:     uint256.NewInt(123),
//				Sediment:  uint256.NewInt(123),
//				Timestamp: 123,
//			},
//		},
//	}, {
//		name: "t2",
//		args: args{
//			key: "qwe456",
//			val: &RewardEntry{
//				Address:   []byte("456"),
//				Value:     uint256.NewInt(456),
//				Sediment:  uint256.NewInt(456),
//				Timestamp: 456,
//			},
//		}}, {
//		name: "t3",
//		args: args{
//			key: "qwe789",
//			val: &RewardEntry{
//				Address:   []byte("789"),
//				Value:     uint256.NewInt(789),
//				Sediment:  uint256.NewInt(789),
//				Timestamp: 789,
//			},
//		},
//	}}
//
//	for _, tt := range tests {
//		if err := PutEpochReward(tx, tt.args.key, tt.args.val); (err != nil) != tt.wantErr {
//			t.Errorf("PutReward() error = %v, wantErr %v", err, tt.wantErr)
//		}
//	}
//
//	m, err := GetRewards(tx, "qwe")
//	if err != nil {
//		t.Error(err)
//	}
//
//	fmt.Println(m)
//	t.Log(m)
//}

func OpenDatabase() (kv.RwDB, error) {
	var chainKv kv.RwDB
	var err error
	logger := log2.New()

	dbPath := "./mdbx.db"

	var openFunc = func(exclusive bool) (kv.RwDB, error) {
		//if config.Http.DBReadConcurrency > 0 {
		//	roTxLimit = int64(config.Http.DBReadConcurrency)
		//}
		roTxsLimiter := semaphore.NewWeighted(int64(cmp.Max(32, runtime.GOMAXPROCS(-1)*8))) // 1 less than max to allow unlocking to happen
		opts := mdbx.NewMDBX(logger).
			WriteMergeThreshold(4 * 8192).
			Path(dbPath).Label(kv.ChainDB).
			DBVerbosity(kv.DBVerbosityLvl(2)).RoTxsLimiter(roTxsLimiter)
		if exclusive {
			opts = opts.Exclusive()
		}

		modules.astInit()
		kv.ChaindataTablesCfg = modules.astTableCfg

		opts = opts.MapSize(8 * datasize.TB)
		return opts.Open()
	}
	chainKv, err = openFunc(false)
	if err != nil {
		return nil, err
	}

	if err = chainKv.Update(context.Background(), func(tx kv.RwTx) (err error) {
		return params.SetastVersion(tx, params.VersionKeyCreated)
	}); err != nil {
		return nil, err
	}
	return chainKv, nil
}
