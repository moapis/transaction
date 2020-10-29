// Copyright (c) 2020, Mohlmann Solutions SRL. All rights reserved.
// Use of this source code is governed by a License that can be found in the LICENSE file.
// SPDX-License-Identifier: BSD-3-Clause

package transaction

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/moapis/multidb"
	"github.com/moapis/multidb/drivers/postgresql"
	"github.com/sirupsen/logrus"

	_ "github.com/lib/pq"
)

var (
	log = logrus.New()
	mdb *multidb.MultiDB
)

func TestMain(m *testing.M) {
	mdb = &multidb.MultiDB{
		MasterFunc: multidb.IsMaster(postgresql.MasterQuery),
	}

	db, err := sql.Open("postgres", "host=localhost dbname=postgres user=postgres ssl_mode=disable connect_timeout=5")
	if err != nil {
		log.WithError(err).Fatal("mdb Open")
	}

	mdb.Add("localhost", db)

	code := m.Run()

	if err = mdb.Close(); err != nil {
		log.WithError(err).Fatal("mdb close")
	}

	os.Exit(code)
}

func TestRequest_errCallback(t *testing.T) {
	rt := &Request{
		Log: logrus.NewEntry(log),
	}

	errs := []error{
		nil,
		sql.ErrNoRows, //ignoredCallbackErrs
		new(multidb.NodeError),
		errors.New("Spanac"),
	}

	for _, err := range errs {
		rt.errCallback(err)
	}
}

func TestNew(t *testing.T) {
	ectx, cancel := context.WithCancel(context.Background())
	cancel()

	type args struct {
		ctx      context.Context
		mdb      *multidb.MultiDB
		routines []int
		log      *logrus.Entry
		readOnly bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"Readonly; nil routines",
			args{
				context.Background(),
				mdb,
				nil,
				logrus.NewEntry(log),
				true,
			},
			false,
		},
		{
			"Readonly",
			args{
				context.Background(),
				mdb,
				[]int{2},
				logrus.NewEntry(log),
				true,
			},
			false,
		},
		{
			"Not Readonly",
			args{
				context.Background(),
				mdb,
				nil,
				logrus.NewEntry(log),
				false,
			},
			false,
		},
		{
			"Context error",
			args{
				ectx,
				mdb,
				nil,
				logrus.NewEntry(log),
				true,
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.args.ctx, tt.args.log, tt.args.mdb, tt.args.readOnly, tt.args.routines...)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Errorf("New() = %v, want %v", got, "something")
			}
		})
	}
}

func Test_requestTx_EnoughTime(t *testing.T) {
	ex, cancel := context.WithTimeout(context.Background(), -1)
	defer cancel()
	short, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	tests := []struct {
		name    string
		ctx     context.Context
		need    time.Duration
		wantErr bool
	}{
		{
			"Expired context",
			ex,
			0,
			true,
		},
		{
			"Enough time",
			short,
			time.Millisecond,
			false,
		},
		{
			"Skip check",
			short,
			time.Millisecond,
			false,
		},
		{
			"Not enough time",
			short,
			2 * time.Second,
			true,
		},
		{
			"No deadline",
			context.Background(),
			time.Millisecond,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := &Request{
				Ctx: tt.ctx,
				Log: logrus.NewEntry(log),
			}
			if err := rt.EnoughTime(tt.need); (err != nil) != tt.wantErr {
				t.Errorf("requestTx.enoughTime() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_requestTx_Done_Commit(t *testing.T) {
	tx, err := New(context.Background(), logrus.NewEntry(log), mdb, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Errorf("requestTx.commit() error = %v, wantErr %v", err, false)
	}
	if err = tx.Commit(); err == nil {
		t.Errorf("requestTx.commit() error = %v, wantErr %v", err, true)
	}
	tx.Done()
	tx, err = New(context.Background(), logrus.NewEntry(log), mdb, false)
	if err != nil {
		t.Fatal(err)
	}
	tx.Done()
}
