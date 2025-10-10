package s3z

import (
	"context"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithy "github.com/aws/smithy-go/endpoints"
)

// OSSEndpointResolver is a S3 resolver for OSS.
type OSSEndpointResolver struct {
	domain string
}

func NewOSSEndpointResolver(domain string) *OSSEndpointResolver {
	return &OSSEndpointResolver{
		domain: domain,
	}
}

// ResolveEndpoint constructs virtual-hosted URL.
func (r *OSSEndpointResolver) ResolveEndpoint(ctx context.Context, p s3.EndpointParameters,
) (smithy.Endpoint, error) {
	bucket := strings.ToLower(*p.Bucket) // Bucket names are case-insensitive
	s := "https://" + bucket + "." + r.domain

	u, err := url.Parse(s)
	if err != nil {
		return smithy.Endpoint{}, err
	}

	return smithy.Endpoint{
		URI: *u,
	}, nil
}
