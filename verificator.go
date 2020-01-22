package transaction

import (
	"context"
	"errors"
	"time"

	auth "github.com/moapis/authenticator"
	"github.com/moapis/authenticator/verify"
	"github.com/moapis/multidb"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Verificator can be used to open authenticated transactions.
type Verificator struct {
	verify.Verificator
}

func attemptDial(ctx context.Context, entry *logrus.Entry, target string) (acc *grpc.ClientConn, err error) {
	for acc == nil {
		if err = ctx.Err(); err != nil {
			return nil, err
		}

		// Local context enables retrying after every 5 seconds
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		acc, err = grpc.DialContext(ctx, target, grpc.WithBlock(), grpc.WithInsecure())
		switch err {
		case nil:
		case context.DeadlineExceeded:
			entry.WithError(err).Warn("gRPC Dial")
		default:
			return nil, err
		}
	}

	return acc, nil
}

// NewVerificator dials the target authentication gRPC server and keeps retrying untill the context expires.
func NewVerificator(ctx context.Context, entry *logrus.Entry, target string, audiences ...string) (*Verificator, error) {
	entry = entry.WithField("target", target)
	entry.Info("Start gRPC Dial")
	acc, err := attemptDial(ctx, entry, target)
	if err != nil {
		entry.WithError(err).Error("gRPC Dial failed")
		return nil, err
	}
	entry.Info("gRPC Dial done")

	return &Verificator{
		verify.Verificator{
			Client:    auth.NewAuthenticatorClient(acc),
			Audiences: audiences,
		},
	}, nil
}

// NewAuth opens an authenticated transaction.
func (v *Verificator) NewAuth(ctx context.Context, entry *logrus.Entry, mdb *multidb.MultiDB, readOnly bool, max int, token string, groups ...string) (*Request, error) {
	req, err := New(ctx, entry, mdb, readOnly, max)
	if err != nil {
		return nil, err
	}
	entry = entry.WithField("token", token)

	req.Claims, err = v.Token(ctx, token)
	if err != nil {
		entry.WithError(err).Error("Check token")
		var ve *verify.VerificationErr
		if errors.As(err, &ve) {
			return nil, status.Error(codes.Unauthenticated, "Unauthorized")
		}
		return nil, status.Error(codes.Internal, ErrAuth)
	}

	entry = entry.WithField("claims", req.Claims)

	var claimGroups []string
	if len(groups) > 0 {
		cgi, ok := req.Claims.Set["groups"].([]interface{})
		if ok {
			claimGroups = make([]string, len(cgi))
			for i, g := range cgi {
				if claimGroups[i], ok = g.(string); !ok {
					return nil, status.Error(codes.InvalidArgument, ErrGroup)
				}
			}
		}

		if !ok || !verify.HasAnyEntry(groups, claimGroups) {
			entry.WithFields(logrus.Fields{
				"required": groups,
				"claimed":  claimGroups,
			}).Error(ErrGroup)
			return nil, status.Error(codes.Unauthenticated, ErrGroup)
		}
	}

	return req, nil
}
