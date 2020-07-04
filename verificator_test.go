package transaction

import (
	"context"
	"testing"
	"time"

	auth "github.com/moapis/authenticator"
	"github.com/moapis/multidb"
	"github.com/sirupsen/logrus"
)

func Test_attemptDial(t *testing.T) {
	ectx, cancel := context.WithCancel(context.Background())
	cancel()

	tctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()

	type args struct {
		ctx    context.Context
		entry  *logrus.Entry
		target string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"Context error",
			args{
				ectx,
				log.WithField("test", "Test_attemptDial"),
				"127.0.0.1:8765",
			},
			true,
		},
		{
			"Timeout",
			args{
				tctx,
				log.WithField("test", "Test_attemptDial"),
				"127.0.0.1:12340",
			},
			true,
		},
		{
			"Success",
			args{
				context.Background(),
				log.WithField("test", "Test_attemptDial"),
				"127.0.0.1:8765",
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAcc, err := attemptDial(tt.args.ctx, tt.args.entry, tt.args.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("attemptDial() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && gotAcc == nil {
				t.Errorf("attemptDial() = %v, want %v", gotAcc, "something")
			}
		})
	}
}

func TestNewVerificator(t *testing.T) {
	ectx, cancel := context.WithCancel(context.Background())
	cancel()

	type args struct {
		ctx       context.Context
		entry     *logrus.Entry
		target    string
		audiences []string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"Context error",
			args{
				ectx,
				log.WithField("test", "TestNewVerificator"),
				"127.0.0.1:8765",
				nil,
			},
			true,
		},
		{
			"Success",
			args{
				context.Background(),
				log.WithField("test", "TestNewVerificator"),
				"127.0.0.1:8765",
				nil,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewVerificator(tt.args.ctx, tt.args.entry, tt.args.target, tt.args.audiences...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewVerificator() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Errorf("attemptDial() = %v, want %v", got, "something")
			}
		})
	}
}

func TestVerificator_NewAuth(t *testing.T) {
	entry := log.WithField("test", "TestVerificator_NewAuth")

	vfc, err := NewVerificator(
		context.Background(),
		entry,
		"127.0.0.1:8765",
		"spanac", "authenticator",
	)
	if err != nil {
		t.Fatal(err)
	}

	ar, err := vfc.Client.AuthenticatePwUser(
		context.Background(),
		&auth.UserPassword{
			Email:    "admin@localhost",
			Password: "admin",
		},
	)

	ectx, cancel := context.WithCancel(context.Background())
	cancel()

	type args struct {
		ctx      context.Context
		entry    *logrus.Entry
		mdb      *multidb.MultiDB
		readOnly bool
		max      int
		token    string
		groups   []string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"Context error",
			args{
				ectx,
				entry,
				mdb,
				true,
				2,
				ar.GetJwt(),
				nil,
			},
			true,
		},
		{
			"Success w/o groups",
			args{
				context.Background(),
				entry,
				mdb,
				true,
				2,
				ar.GetJwt(),
				nil,
			},
			false,
		},
		{
			"Success with groups",
			args{
				context.Background(),
				entry,
				mdb,
				true,
				2,
				ar.GetJwt(),
				[]string{"primary", "spanac"},
			},
			false,
		},
		{
			"Token error",
			args{
				context.Background(),
				entry,
				mdb,
				true,
				2,
				"Fooo",
				[]string{"primary", "spanac"},
			},
			true,
		},
		{
			"Wrong group",
			args{
				context.Background(),
				entry,
				mdb,
				true,
				2,
				ar.GetJwt(),
				[]string{"spanac"},
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := vfc.NewAuth(tt.args.ctx, tt.args.entry, tt.args.mdb, tt.args.readOnly, tt.args.max, tt.args.token, tt.args.groups...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Verificator.NewAuth() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Errorf("Verificator.NewAuth() = %v, want %v", got, "something")
			}
		})
	}
}
