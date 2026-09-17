// SPDX-FileCopyrightText: 2026 crossplane-contrib
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	ujconfig "github.com/crossplane/upjet/v2/pkg/config"
	"github.com/crossplane/upjet/v2/pkg/pipeline"
	harborprovider "github.com/goharbor/terraform-provider-harbor/provider"

	"github.com/crossplane-contrib/provider-upjet-harbor/config"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] == "" {
		panic("root directory is required to be given as argument")
	}
	rootDir := os.Args[1]
	absRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		panic(fmt.Sprintf("cannot calculate the absolute path with %s", rootDir))
	}

	ctx := context.Background()
	sdkProvider := harborprovider.Provider()

	pc, err := config.GetProvider(ctx, sdkProvider)
	if err != nil {
		panic(fmt.Sprintf("cannot initialize the cluster-scoped provider configuration: %v", err))
	}
	pns, err := config.GetProviderNamespaced(ctx, sdkProvider)
	if err != nil {
		panic(fmt.Sprintf("cannot initialize the namespaced provider configuration: %v", err))
	}
	dumpGeneratedResourceList(pc, new("../config/generated.lst"))

	pipeline.Run(pc, pns, absRootDir)
}

func dumpGeneratedResourceList(p *ujconfig.Provider, targetPath *string) {
	if len(*targetPath) == 0 {
		return
	}
	generatedResources := make([]string, 0, len(p.Resources))
	for name := range p.Resources {
		generatedResources = append(generatedResources, name)
	}
	sort.Strings(generatedResources)
	buff, err := json.MarshalIndent(generatedResources, "", "")
	if err != nil {
		panic(fmt.Sprintf("Cannot marshal native schema versions to JSON: %s", err.Error()))
	}
	if err := os.WriteFile(*targetPath, buff, 0o600); err != nil {
		panic(fmt.Sprintf("Cannot write native schema versions of generated resources to file %s: %s", *targetPath, err.Error()))
	}
}
