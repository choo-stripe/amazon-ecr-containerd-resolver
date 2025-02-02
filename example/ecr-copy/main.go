/*
 * Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
package main

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/namespaces"
	"github.com/samuelkarp/amazon-ecr-containerd-resolver/ecr"
)

func main() {
	ctx := namespaces.NamespaceFromEnv(context.Background())

	if len(os.Args) < 2 {
		log.G(ctx).Fatal("Must provide source and destination as arguments")
	}

	sourceRef := os.Args[1]
	destRef := os.Args[2]

	client, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		log.G(ctx).WithError(err).Fatal("Failed to connect to containerd")
	}
	defer client.Close()

	awsSession, err := session.NewSession()
	if err != nil {
		log.G(ctx).WithError(err).Fatal("Failed to create AWS session")
	}
	resolver := ecr.NewResolver(awsSession, ecr.ResolverOptions{})

	log.G(ctx).WithField("sourceRef", sourceRef).Info("Pulling from Amazon ECR")
	img, err := client.Pull(
		ctx,
		sourceRef,
		containerd.WithResolver(resolver),
	)
	if err != nil {
		log.G(ctx).WithError(err).WithField("sourceRef", sourceRef).Fatal("Failed to pull")
	}
	log.G(ctx).WithField("img", img.Name()).Info("Pulled successfully!")

	log.G(ctx).WithField("sourceRef", sourceRef).WithField("destRef", destRef).Info("Pushing to Amazon ECR")
	desc := img.Target()
	err = client.Push(ctx, destRef, desc,
		containerd.WithResolver(resolver),
	)
	if err != nil {
		log.G(ctx).WithError(err).WithField("destRef", destRef).Fatal("Failed to push")
	}

	log.G(ctx).WithField("destRef", destRef).Info("Pushed successfully!")
}
