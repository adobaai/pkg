package ratemw

import (
	"maps"
	"strconv"

	kerrors "github.com/go-kratos/kratos/v2/errors"
)

var (
	ErrNoClaims               = kerrors.Unauthorized("UNAUTHORIZED", "missing auth claims")
	ErrRPMLimitExceeded       = kerrors.New(429, "TOO_MANY_REQUESTS", "rate limit exceeded")
	ErrInvalidUserIDExtractor = kerrors.InternalServer("INTERNAL", "nil user id extractor")
)

func WithRetryAfter(err error, seconds int) error {
	if err == nil {
		return nil
	}
	if seconds < 1 {
		seconds = 1
	}

	src := kerrors.FromError(err)
	md := map[string]string{}
	maps.Copy(md, src.Metadata)
	md["retry_after"] = strconv.Itoa(seconds)

	return kerrors.New(int(src.Code), src.Reason, src.Message).WithMetadata(md)
}
