// Copyright (c) 2020, Mohlmann Solutions SRL. All rights reserved.
// Use of this source code is governed by a License that can be found in the LICENSE file.
// SPDX-License-Identifier: BSD-3-Clause

// Package transaction provides a reusable component for creating and closing a request scope in gRPC.
package transaction

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moapis/multidb"
	"github.com/pascaldekloe/jwt"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/boil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Request holds a database transaction, request context and a logrus entry.
type Request struct {
	Ctx      context.Context
	Tx       boil.ContextTransactor
	Log      *logrus.Entry
	ReadOnly bool
	Claims   *jwt.Claims

	cancel context.CancelFunc
}

// New Request opens a transaction on mdb.
// If the transaction is readonly, routines amount of Nodes will be requested.
func New(ctx context.Context, log *logrus.Entry, mdb *multidb.MultiDB, readOnly bool, routines ...int) (*Request, error) {
	rt := &Request{
		Log:      log,
		ReadOnly: readOnly,
	}
	var err error
	if readOnly {
		max := 1
		if len(routines) > 0 {
			max = routines[0]
		}
		rt.Tx, err = mdb.MultiTx(ctx, &sql.TxOptions{ReadOnly: readOnly}, max)
	} else {
		rt.Tx, err = mdb.MasterTx(ctx, nil)
	}
	if err != nil {
		rt.Log.WithError(err).Error("Begin TX")
		return nil, err
	}
	rt.Log.Debug("Begin TX")
	rt.Ctx, rt.cancel = context.WithCancel(ctx)
	return rt, nil
}

const (
	// ErrNotEnoughTime is returned as a gRPC error message.
	ErrNotEnoughTime = "Not enough time in context"
	// ErrDB is returned as a gRPC error message.
	ErrDB = "Database error"
	// ErrAuth is returned as a gRPC error message.
	ErrAuth = "Authentication server error"
	// ErrGroup is returned as a gRPC error message.
	ErrGroup = "User not in required group"
)

// EnoughTime checks if the context is valid and has enough time available.
func (rt *Request) EnoughTime(need time.Duration) error {
	if err := rt.Ctx.Err(); err != nil {
		rt.Log.WithError(err).Warn("enoughTime")
		return status.FromContextError(err).Err()
	}
	dl, ok := rt.Ctx.Deadline()
	ll := rt.Log.WithFields(logrus.Fields{"deadline": dl, "need": need})
	if need != 0 && ok && time.Now().Add(need).After(dl) {
		ll.WithError(errors.New(ErrNotEnoughTime)).Warn("enoughTime")
		return status.Error(codes.Aborted, ErrNotEnoughTime)
	}
	ll.Debug("enoughTime")
	return nil
}

// Done Rollbacks an open transaction and cancels the Request Context.
// Done is meant to be deferred and errors are logged, not returned.
func (rt *Request) Done() {
	err := rt.Tx.Rollback()
	rt.cancel()

	if err != nil {
		rt.Log.WithError(err).Error("TX Rollback")
	} else {
		rt.Log.Debug("TX Rollback")
	}
}

// Commit the current transaction
func (rt *Request) Commit() error {
	err := rt.Tx.Commit()
	if err != nil {
		rt.Log.WithError(err).Error("TX commit")
		return status.Error(codes.Internal, ErrDB)
	}
	rt.Log.Debug("TX Commit")
	return nil
}
