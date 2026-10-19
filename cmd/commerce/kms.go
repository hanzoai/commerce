// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/hanzoai/kms/sdk/go/kmsclient"

	commerce "github.com/hanzoai/commerce"
)

// secretReader is a standalone commerce's secret plane: Hanzo KMS through
// hanzoai/kms/sdk/go, as the deployment's machine identity. Embedded in the
// cloud binary, commerce reads through the host's own KMS handle instead
// (EmbedConfig.Secrets); this is the same reader for a binary with no host.
//
// KMS_URL names the server; unset, there is no reader and commerce reads no
// secret. Set, the identity is required — KMS_CLIENT_ID, KMS_CLIENT_SECRET and
// KMS_ORG, exchanged at HANZO_IAM_URL — and a configuration that cannot make a
// client refuses to boot rather than serving with no credentials.
func secretReader() (commerce.SecretReader, error) {
	endpoint := strings.TrimSpace(os.Getenv("KMS_URL"))
	if endpoint == "" {
		return nil, nil
	}
	org := strings.TrimSpace(os.Getenv("KMS_ORG"))
	if org == "" {
		return nil, errors.New("KMS_URL is set and KMS_ORG is not: the reader serves one org's material and must know which")
	}
	iam := strings.TrimSpace(os.Getenv("HANZO_IAM_URL"))
	if iam == "" {
		iam = "https://hanzo.id"
	}
	c, err := kmsclient.New(kmsclient.Config{
		Endpoint:     endpoint,
		IAMEndpoint:  iam,
		ClientID:     strings.TrimSpace(os.Getenv("KMS_CLIENT_ID")),
		ClientSecret: strings.TrimSpace(os.Getenv("KMS_CLIENT_SECRET")),
		Org:          org,
	})
	if err != nil {
		return nil, fmt.Errorf("kms: %w", err)
	}
	return orgReader{c: c, org: org}, nil
}

// orgReader answers refs under its identity's own org, orgs/<org>/<path>/<name>,
// and refuses every other: the server folds a read under the credential's org, so
// a ref naming another org could only ever read this one's.
type orgReader struct {
	c   *kmsclient.Client
	org string
}

func (r orgReader) GetSecret(ctx context.Context, ref string) ([]byte, error) {
	rest, ok := strings.CutPrefix(ref, "orgs/"+r.org+"/")
	if !ok {
		return nil, fmt.Errorf("kms: %q is not under orgs/%s", ref, r.org)
	}
	path, name := "", rest
	if i := strings.LastIndex(rest, "/"); i >= 0 {
		path, name = rest[:i], rest[i+1:]
	}
	v, err := r.c.Get(ctx, path, name)
	if err != nil {
		return nil, err
	}
	return []byte(v), nil
}
