// SPDX-FileCopyrightText: 2026 crossplane-contrib
// SPDX-License-Identifier: Apache-2.0

package apis

import (
	xpresource "github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var s = runtime.NewScheme()

// GetManagedResource returns a new managed and managed-list object for
// the supplied GVK using the module's private resolver scheme.
func GetManagedResource(group, version, kind, listKind string) (xpresource.Managed, xpresource.ManagedList, error) {
	gv := schema.GroupVersion{Group: group, Version: version}

	mObj, err := s.New(gv.WithKind(kind))
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to get a new API object of GVK %q from the runtime scheme", gv.WithKind(kind))
	}
	lObj, err := s.New(gv.WithKind(listKind))
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to get a new API object list of GVK %q from the runtime scheme", gv.WithKind(listKind))
	}
	return mObj.(xpresource.Managed), lObj.(xpresource.ManagedList), nil
}

// BuildScheme registers the supplied SchemeBuilder with the module's
// private resolver scheme.
func BuildScheme(sb runtime.SchemeBuilder) error {
	return errors.Wrap(sb.AddToScheme(s), "failed to register the GVKs with the runtime scheme")
}
