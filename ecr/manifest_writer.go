/*
 * Copyright 2017-2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"). You
 * may not use this file except in compliance with the License. A copy of
 * the License is located at
 *
 * 	http://aws.amazon.com/apache2.0/
 *
 * or in the "license" file accompanying this file. This file is
 * distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
 * ANY KIND, either express or implied. See the License for the specific
 * language governing permissions and limitations under the License.
 */
package ecr

import (
	"bytes"
	"context"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type manifestWriter struct {
	ctx     context.Context
	base    *ecrBase
	desc    ocispec.Descriptor
	buf     bytes.Buffer
	tracker docker.StatusTracker
	ref     string
}

var _ content.Writer = (*manifestWriter)(nil)

func (mw *manifestWriter) Write(b []byte) (int, error) {
	log.G(mw.ctx).WithField("len(b)", len(b)).Debug("ecr.manifest.write")
	return mw.buf.Write(b)
}

func (mw *manifestWriter) Close() error {
	return errors.New("ecr.manifest.close: not implemented")
}

func (mw *manifestWriter) Digest() digest.Digest {
	return mw.desc.Digest
}

func (mw *manifestWriter) Commit(ctx context.Context, size int64, expected digest.Digest, opts ...content.Opt) error {
	log.G(mw.ctx).WithField("size", size).WithField("expected", expected).Debug("ecr.manifest.commit")
	manifest := mw.buf.String()
	log.G(mw.ctx).WithField("manifest", manifest).Debug("ecr.manifest.commit")
	ecrSpec := mw.base.ecrSpec
	tag, _ := ecrSpec.TagDigest()
	putImageInput := &ecr.PutImageInput{
		RegistryId:     aws.String(ecrSpec.Registry()),
		RepositoryName: aws.String(ecrSpec.Repository),
		ImageTag:       aws.String(tag),
		ImageManifest:  aws.String(manifest),
	}

	output, err := mw.base.client.PutImageWithContext(ctx, putImageInput)
	if err != nil {
		return errors.Wrapf(err, "ecr: failed to put manifest: %v", ecrSpec)
	}

	status, err := mw.tracker.GetStatus(mw.ref)
	if err == nil {
		status.Offset = int64(len(manifest))
		status.UpdatedAt = time.Now()
		mw.tracker.SetStatus(mw.ref, status)
	} else {
		log.G(mw.ctx).WithError(err).WithField("ref", mw.ref).Warn("Failed to update status")
	}

	if output == nil {
		return errors.Errorf("ecr: failed to put manifest, nil output: %v", ecrSpec)
	}
	actual := aws.StringValue(output.Image.ImageId.ImageDigest)
	if actual != expected.String() {
		return errors.Errorf("got digest %s, expected %s", actual, expected)
	}
	return nil
}

func (mw *manifestWriter) Status() (content.Status, error) {
	log.G(mw.ctx).Debug("ecr.manifest.status")

	status, err := mw.tracker.GetStatus(mw.ref)
	if err != nil {
		return content.Status{}, err
	}
	return status.Status, nil
}

func (mw *manifestWriter) Truncate(size int64) error {
	log.G(mw.ctx).WithField("size", size).Debug("ecr.manifest.truncate")
	return errors.New("mw.Truncate: not implemented")
}
